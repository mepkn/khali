// Package scan finds projects under the chosen folders and the regenerable
// folders inside them (node_modules, build output, generated native projects).
package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"khali/internal/fsx"
)

// Target is one folder (or file) that can be deleted and regenerated.
type Target struct {
	Path  string `json:"path"`
	Rel   string `json:"rel"` // relative to the project
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Regen string `json:"regen"` // how to get it back
	Bytes int64  `json:"bytes"`
	Files int    `json:"files"`
	// Safe is false when git tracks something inside, or git can't tell;
	// such targets are shown but can't be selected.
	Safe bool   `json:"safe"`
	Note string `json:"note"`
}

// Project is a git repo (or a folder with a package.json / go.mod outside one).
type Project struct {
	Path       string       `json:"path"`
	Name       string       `json:"name"`
	Git        bool         `json:"git"`
	LastActive int64        `json:"lastActive"` // unix seconds
	Targets    []Target     `json:"targets"`
	Bytes      int64        `json:"bytes"` // sum of safe targets
	Android    []AndroidApp `json:"android"`
}

type Progress struct {
	Phase   string `json:"phase"` // "finding", "sizing"
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Current string `json:"current"`
}

type Options struct {
	Roots    []string
	Versions VersionTable // remembered + seed versions
	Progress func(Progress)
}

type Result struct {
	Projects  []Project `json:"projects"`
	ScannedAt int64     `json:"scannedAt"`
	// Observed are versions read from real files this scan, by RN minor.
	Observed VersionTable `json:"-"`
}

// maxDepth bounds both the search for projects and the walk inside one.
const (
	maxRootDepth    = 4
	maxProjectDepth = 6
)

func Run(opts Options) Result {
	progress := opts.Progress
	if progress == nil {
		progress = func(Progress) {}
	}

	var paths []string
	seen := map[string]bool{}
	for _, root := range opts.Roots {
		for _, p := range FindProjects(root) {
			if !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}

	res := Result{Observed: VersionTable{}}
	for i, p := range paths {
		progress(Progress{Phase: "finding", Done: i, Total: len(paths), Current: filepath.Base(p)})
		proj := collect(p, opts.Versions, res.Observed)
		res.Projects = append(res.Projects, proj)
	}

	// Size every target with a few workers; node_modules walks dominate.
	type job struct{ p, t int }
	var jobs []job
	for pi := range res.Projects {
		for ti := range res.Projects[pi].Targets {
			jobs = append(jobs, job{pi, ti})
		}
	}
	sizer := fsx.NewSizer()
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done int
		ch   = make(chan job)
	)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				t := &res.Projects[j.p].Targets[j.t]
				u := sizer.Size(t.Path)
				t.Bytes, t.Files = u.Bytes, u.Files
				mu.Lock()
				done++
				progress(Progress{Phase: "sizing", Done: done, Total: len(jobs), Current: res.Projects[j.p].Name + "/" + t.Rel})
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()

	for pi := range res.Projects {
		p := &res.Projects[pi]
		sort.Slice(p.Targets, func(a, b int) bool { return p.Targets[a].Bytes > p.Targets[b].Bytes })
		for _, t := range p.Targets {
			if t.Safe {
				p.Bytes += t.Bytes
			}
		}
	}
	sort.Slice(res.Projects, func(a, b int) bool { return res.Projects[a].Bytes > res.Projects[b].Bytes })
	res.ScannedAt = time.Now().Unix()
	return res
}

// FindProjects returns the project folders under root: git repos, or folders
// with a package.json / go.mod / app.json that aren't inside a repo.
func FindProjects(root string) []string {
	var out []string
	var visit func(dir string, depth int)
	visit = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		names := map[string]bool{}
		for _, e := range entries {
			names[e.Name()] = true
		}
		if names[".git"] || names["package.json"] || names["go.mod"] || names["app.json"] {
			out = append(out, dir)
			return
		}
		if depth >= maxRootDepth {
			return
		}
		for _, e := range entries {
			n := e.Name()
			if !e.IsDir() || strings.HasPrefix(n, ".") || n == "node_modules" || n == "Library" {
				continue
			}
			visit(filepath.Join(dir, n), depth+1)
		}
	}
	visit(root, 0)
	return out
}

// collect walks one project, applies the rules and the git safety checks.
func collect(root string, table VersionTable, observed VersionTable) Project {
	p := Project{Path: root, Name: filepath.Base(root), Git: isRealDir(filepath.Join(root, ".git"))}
	var cands []candidate
	var appDirs []string

	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		d, entries, err := readDirInfo(dir)
		if err != nil {
			return
		}
		if d.hasDep("react-native") {
			appDirs = append(appDirs, dir)
		}
		for _, e := range entries {
			found, descend := match(d, e)
			cands = append(cands, found...)
			if descend && depth < maxProjectDepth {
				walk(filepath.Join(dir, e.Name()), depth+1)
			}
		}
	}
	walk(root, 0)

	p.Targets = resolveSafety(root, p.Git, cands)
	p.LastActive = lastActive(root, p.Git, p.Targets)

	for _, dir := range appDirs {
		app, obs, ok := readAndroidApp(dir, table)
		if !ok {
			continue
		}
		app.Project = p.Name
		app.Rel = relTo(root, dir)
		p.Android = append(p.Android, app)
		if minor := rnMinor(app.RN); minor != "" {
			observed[minor] = observed[minor].fill(obs)
		}
	}
	return p
}

// resolveSafety keeps a candidate when git ignores it and tracks nothing in
// it. A rejected candidate with a fallback (whole android/) is replaced by
// its fallback (the build folders). Outside git every candidate is allowed:
// the rules only match regenerable folders.
func resolveSafety(root string, isGit bool, cands []candidate) []Target {
	var all []string
	for _, c := range cands {
		all = append(all, relTo(root, c.Path))
		for _, f := range c.fallback {
			all = append(all, relTo(root, f.Path))
		}
	}
	var ignored, tracked map[string]bool
	gitErr := ""
	if isGit {
		var err1, err2 error
		ignored, err1 = gitIgnored(root, all)
		tracked, err2 = gitTracked(root, all)
		if err1 != nil || err2 != nil {
			gitErr = "git couldn't check this folder"
		}
	}

	check := func(c candidate) Target {
		t := c.Target
		t.Rel = relTo(root, t.Path)
		switch {
		case !isGit && c.needsGit:
			t.Note = "not in a git repo, so it can't be confirmed as build output"
		case !isGit:
			t.Safe, t.Note = true, "not in a git repo"
		case gitErr != "":
			t.Note = gitErr
		case tracked[t.Rel]:
			t.Note = "git tracks files in here"
		case !ignored[t.Rel]:
			t.Note = "not in .gitignore"
		default:
			t.Safe = true
		}
		return t
	}

	var out []Target
	for _, c := range cands {
		t := check(c)
		if !t.Safe && len(c.fallback) > 0 {
			for _, f := range c.fallback {
				ft := check(f)
				ft.Note = strings.TrimSpace(ft.Note + " (" + t.Rel + " is kept: " + t.Note + ")")
				out = append(out, ft)
			}
			continue
		}
		out = append(out, t)
	}
	return out
}

// lastActive estimates when the project was last worked on: the newest of
// HEAD's commit, git's index, and file edits outside the cleanable folders.
// Folder times are ignored so that cleaning doesn't make a project look used.
func lastActive(root string, isGit bool, targets []Target) int64 {
	var newest int64
	if isGit {
		newest = gitLastCommit(root)
		if info, err := os.Stat(filepath.Join(root, ".git", "index")); err == nil {
			newest = max(newest, info.ModTime().Unix())
		}
	}
	skip := map[string]bool{}
	for _, t := range targets {
		skip[t.Path] = true
	}
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			n := e.Name()
			path := filepath.Join(dir, n)
			if skip[path] || strings.HasPrefix(n, ".") || n == "node_modules" {
				continue
			}
			if e.IsDir() {
				if depth < 3 {
					walk(path, depth+1)
				}
				continue
			}
			if info, err := e.Info(); err == nil {
				newest = max(newest, info.ModTime().Unix())
			}
		}
	}
	walk(root, 0)
	return newest
}
