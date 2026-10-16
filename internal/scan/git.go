package scan

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// gitIgnored returns the subset of rels (paths relative to repo) that git
// ignores. Paths are checked on disk, so a folder matches "dir/" patterns.
func gitIgnored(repo string, rels []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(rels) == 0 {
		return out, nil
	}
	cmd := exec.Command("git", "-C", repo, "check-ignore", "--stdin", "-z")
	cmd.Stdin = strings.NewReader(strings.Join(rels, "\x00") + "\x00")
	b, err := cmd.Output()
	// Exit status 1 means none of the paths are ignored.
	if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, p := range bytes.Split(b, []byte{0}) {
		if len(p) > 0 {
			out[string(p)] = true
		}
	}
	return out, nil
}

// gitTracked returns the rels that contain (or are) at least one file git
// tracks. Such a folder is never deleted, even if it also matches .gitignore.
func gitTracked(repo string, rels []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(rels) == 0 {
		return out, nil
	}
	args := append([]string{"-C", repo, "ls-files", "-z", "--"}, rels...)
	b, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, err
	}
	for _, f := range bytes.Split(b, []byte{0}) {
		if len(f) == 0 {
			continue
		}
		file := string(f)
		for _, rel := range rels {
			if file == rel || strings.HasPrefix(file, rel+"/") {
				out[rel] = true
			}
		}
	}
	return out, nil
}

// gitLastCommit is the time of HEAD's commit in unix seconds, 0 if none.
func gitLastCommit(repo string) int64 {
	b, err := exec.Command("git", "-C", repo, "log", "-1", "--format=%ct").Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n
}

func relTo(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
