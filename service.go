package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"khali/internal/caches"
	"khali/internal/fsx"
	"khali/internal/scan"
	"khali/internal/store"
)

// Khali is the service the frontend calls. It keeps the last scan so that
// cleaning only ever removes paths a scan found and marked safe.
type Khali struct {
	store *store.Store
	home  string

	mu       sync.Mutex // guards the fields below
	last     *scan.Result
	cacheLst []caches.Item
	busy     bool
	onChange func() // refreshes the tray after a clean
}

type Disk struct {
	Total int64 `json:"total"`
	Free  int64 `json:"free"`
}

type CleanError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type CleanReport struct {
	Freed   int64        `json:"freed"`
	Removed []string     `json:"removed"`
	Errors  []CleanError `json:"errors"`
	Before  Disk         `json:"before"`
	After   Disk         `json:"after"`
}

type CleanProgress struct {
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Current string `json:"current"`
}

func init() {
	application.RegisterEvent[scan.Progress]("scan:progress")
	application.RegisterEvent[CleanProgress]("clean:progress")
}

func NewKhali() (*Khali, error) {
	st, err := store.Open()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Khali{store: st, home: home}, nil
}

func (k *Khali) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	return nil
}

func (k *Khali) GetSettings() store.Settings {
	return k.store.Settings()
}

func (k *Khali) SaveSettings(s store.Settings) error {
	var roots []string
	for _, r := range s.Roots {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if !fsx.IsDir(r) {
			return fmt.Errorf("%s is not a folder", r)
		}
		roots = append(roots, filepath.Clean(r))
	}
	s.Roots = roots
	if s.Pinned == nil {
		s.Pinned = []string{}
	}
	return k.store.SaveSettings(s)
}

// SetPinned pins or unpins one project without a full settings round trip.
func (k *Khali) SetPinned(path string, pinned bool) (store.Settings, error) {
	s := k.store.Settings()
	var out []string
	for _, p := range s.Pinned {
		if p != path {
			out = append(out, p)
		}
	}
	if pinned {
		out = append(out, path)
	}
	s.Pinned = out
	return s, k.SaveSettings(s)
}

// PickFolder asks for a folder to add to the scanned roots.
func (k *Khali) PickFolder() (string, error) {
	return application.Get().Dialog.OpenFile().
		SetTitle("Choose a folder with projects").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(false).
		SetDirectory(k.home).
		PromptForSingleSelection()
}

func (k *Khali) begin() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.busy {
		return errors.New("a scan or clean is already running")
	}
	k.busy = true
	return nil
}

func (k *Khali) end() {
	k.mu.Lock()
	k.busy = false
	k.mu.Unlock()
}

// LastScan returns the previous scan of this session, or nil.
func (k *Khali) LastScan() *scan.Result {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.last
}

// Scan finds projects under the roots and sizes their cleanable folders.
func (k *Khali) Scan() (*scan.Result, error) {
	if err := k.begin(); err != nil {
		return nil, err
	}
	defer k.end()
	res := k.scan()
	return &res, nil
}

func (k *Khali) scan() scan.Result {
	app := application.Get()
	res := scan.Run(scan.Options{
		Roots:    k.store.Settings().Roots,
		Versions: k.store.Versions(),
		Progress: throttle(func(p scan.Progress) { app.Event.Emit("scan:progress", p) }),
	})
	_ = k.store.RememberVersions(res.Observed)
	k.mu.Lock()
	k.last = &res
	k.mu.Unlock()
	return res
}

// ScanCaches lists the shared caches. It needs the projects' Android
// versions, so it scans the projects first when there is no scan yet.
func (k *Khali) ScanCaches() ([]caches.Item, error) {
	if err := k.begin(); err != nil {
		return nil, err
	}
	defer k.end()
	last := k.LastScan()
	if last == nil {
		res := k.scan()
		last = &res
	}
	var apps []scan.AndroidApp
	for _, p := range last.Projects {
		apps = append(apps, p.Android...)
	}
	items := caches.List(k.home, apps)
	k.mu.Lock()
	k.cacheLst = items
	k.mu.Unlock()
	return items, nil
}

// CleanTargets removes project folders by path. Each path must be a safe
// target of the last scan, in a project that isn't pinned.
func (k *Khali) CleanTargets(paths []string) (CleanReport, error) {
	if err := k.begin(); err != nil {
		return CleanReport{}, err
	}
	defer k.end()

	k.mu.Lock()
	last := k.last
	k.mu.Unlock()
	if last == nil {
		return CleanReport{}, errors.New("scan first")
	}
	settings := k.store.Settings()
	pinned := map[string]bool{}
	for _, p := range settings.Pinned {
		pinned[p] = true
	}
	known := map[string]scan.Target{}
	for _, p := range last.Projects {
		if pinned[p.Path] {
			continue
		}
		for _, t := range p.Targets {
			if t.Safe {
				known[t.Path] = t
			}
		}
	}

	var jobs []job
	for _, p := range paths {
		t, ok := known[p]
		if !ok {
			return CleanReport{}, fmt.Errorf("%s is not a cleanable folder from the last scan", p)
		}
		jobs = append(jobs, job{path: t.Path, label: t.Label, regen: t.Regen, bytes: t.Bytes})
	}
	rep := k.run(jobs, settings.Trash)

	// Drop what was removed from the cached scan.
	removed := map[string]bool{}
	for _, p := range rep.Removed {
		removed[p] = true
	}
	k.mu.Lock()
	for pi := range k.last.Projects {
		proj := &k.last.Projects[pi]
		var keep []scan.Target
		proj.Bytes = 0
		for _, t := range proj.Targets {
			if removed[t.Path] {
				continue
			}
			keep = append(keep, t)
			if t.Safe {
				proj.Bytes += t.Bytes
			}
		}
		proj.Targets = keep
	}
	k.mu.Unlock()
	return rep, nil
}

// CleanCaches removes shared caches by id from the last ScanCaches.
func (k *Khali) CleanCaches(ids []string) (CleanReport, error) {
	if err := k.begin(); err != nil {
		return CleanReport{}, err
	}
	defer k.end()

	k.mu.Lock()
	known := map[string]caches.Item{}
	for _, it := range k.cacheLst {
		known[it.ID] = it
	}
	k.mu.Unlock()

	var jobs []job
	for _, id := range ids {
		it, ok := known[id]
		if !ok || it.Status == caches.InUse {
			return CleanReport{}, fmt.Errorf("%s can't be cleaned", id)
		}
		jobs = append(jobs, job{path: it.Path, id: it.ID, label: it.Group + ": " + it.Label, regen: it.Note, bytes: it.Bytes, command: it.Command})
	}
	rep := k.run(jobs, k.store.Settings().Trash)

	removed := map[string]bool{}
	for _, id := range rep.Removed {
		removed[id] = true
	}
	k.mu.Lock()
	var keep []caches.Item
	for _, it := range k.cacheLst {
		if !removed[it.ID] {
			keep = append(keep, it)
		}
	}
	k.cacheLst = keep
	k.mu.Unlock()
	return rep, nil
}

type job struct {
	path, id, label, regen string
	bytes                  int64
	command                []string
}

// run removes each job, re-checking the path right before touching it.
func (k *Khali) run(jobs []job, toTrash bool) CleanReport {
	app := application.Get()
	rep := CleanReport{Before: k.Disk(), Removed: []string{}, Errors: []CleanError{}}
	var log []store.LogEntry
	for i, j := range jobs {
		name := j.path
		if name == "" {
			name = j.label
		}
		app.Event.Emit("clean:progress", CleanProgress{Done: i, Total: len(jobs), Current: name})
		mode, err := k.remove(j, toTrash)
		key := j.path
		if j.id != "" {
			key = j.id
		}
		if err != nil {
			rep.Errors = append(rep.Errors, CleanError{Path: name, Error: err.Error()})
			continue
		}
		rep.Removed = append(rep.Removed, key)
		rep.Freed += j.bytes
		log = append(log, store.LogEntry{Time: time.Now().Unix(), Path: name, Label: j.label, Bytes: j.bytes, Regen: j.regen, Mode: mode})
	}
	app.Event.Emit("clean:progress", CleanProgress{Done: len(jobs), Total: len(jobs)})
	_ = k.store.AppendLog(log)
	rep.After = k.Disk()
	if k.onChange != nil {
		k.onChange()
	}
	return rep
}

func (k *Khali) remove(j job, toTrash bool) (string, error) {
	if len(j.command) > 0 {
		out, err := exec.Command(j.command[0], j.command[1:]...).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
		}
		return "command", nil
	}
	if err := k.checkPath(j.path); err != nil {
		return "", err
	}
	if toTrash {
		return "trash", fsx.Trash(j.path)
	}
	return "delete", fsx.Remove(j.path)
}

// checkPath is the last guard before a delete: an absolute path inside the
// home folder, at least two levels below it, and not a symlink.
func (k *Khali) checkPath(p string) error {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return errors.New("not a clean absolute path")
	}
	rel, err := filepath.Rel(k.home, p)
	if err != nil || strings.HasPrefix(rel, "..") || !strings.Contains(rel, string(filepath.Separator)) {
		return errors.New("refusing to remove a path outside or at the top of the home folder")
	}
	info, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("is a symlink")
	}
	return nil
}

// Disk reports the startup volume's size and free space.
func (k *Khali) Disk() Disk {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/System/Volumes/Data", &st); err != nil {
		if err := syscall.Statfs("/", &st); err != nil {
			return Disk{}
		}
	}
	return Disk{Total: int64(st.Blocks) * int64(st.Bsize), Free: int64(st.Bavail) * int64(st.Bsize)}
}

// Reveal shows a path in Finder.
func (k *Khali) Reveal(path string) error {
	return exec.Command("open", "-R", path).Run()
}

func (k *Khali) Log() []store.LogEntry {
	return k.store.Log(200)
}

// throttle limits progress events to about 20 a second; the last one of a
// phase always goes through so counters end at their total.
func throttle(f func(scan.Progress)) func(scan.Progress) {
	var last time.Time
	return func(p scan.Progress) {
		if p.Done+1 >= p.Total || time.Since(last) > 50*time.Millisecond {
			last = time.Now()
			f(p)
		}
	}
}
