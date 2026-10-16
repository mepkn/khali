// Package store keeps khali's settings, remembered versions and the cleanup
// log in ~/Library/Application Support/khali.
package store

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"khali/internal/scan"
)

type Settings struct {
	// Roots are the folders searched for projects.
	Roots []string `json:"roots"`
	// Pinned projects (paths) are never cleaned: the one you're working on.
	Pinned []string `json:"pinned"`
	// Trash moves items to the Trash instead of deleting them.
	Trash bool `json:"trash"`
	// RecentDays: projects touched within this many days are flagged as
	// recent and left out of "select all".
	RecentDays int `json:"recentDays"`
}

// LogEntry is one removed item.
type LogEntry struct {
	Time  int64  `json:"time"`
	Path  string `json:"path"`
	Label string `json:"label"`
	Bytes int64  `json:"bytes"`
	Regen string `json:"regen"`
	Mode  string `json:"mode"` // "delete", "trash", "command"
}

type Store struct {
	dir string
	mu  sync.Mutex
}

func Open() (*Store, error) {
	cfg, err := os.UserConfigDir() // ~/Library/Application Support on macOS
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cfg, "khali")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func defaults() Settings {
	home, _ := os.UserHomeDir()
	return Settings{Roots: []string{filepath.Join(home, "Desktop", "praveen")}, RecentDays: 2}
}

func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := defaults()
	readJSON(filepath.Join(s.dir, "settings.json"), &st)
	if st.RecentDays <= 0 {
		st.RecentDays = 2
	}
	if st.Pinned == nil {
		st.Pinned = []string{}
	}
	return st
}

func (s *Store) SaveSettings(st Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSON(filepath.Join(s.dir, "settings.json"), st)
}

// Versions returns the seed table overlaid with versions seen in past scans.
func (s *Store) Versions() scan.VersionTable {
	s.mu.Lock()
	defer s.mu.Unlock()
	saved := scan.VersionTable{}
	readJSON(filepath.Join(s.dir, "versions.json"), &saved)
	out := scan.VersionTable{}
	for k, v := range scan.SeedVersions {
		out[k] = v
	}
	for k, v := range saved {
		out[k] = v
	}
	return out
}

// RememberVersions stores versions read from real files. Fields read this
// scan win; fields not seen keep their remembered value.
func (s *Store) RememberVersions(observed scan.VersionTable) error {
	if len(observed) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.dir, "versions.json")
	saved := scan.VersionTable{}
	readJSON(path, &saved)
	for k, v := range observed {
		old := saved[k]
		if v.NDK != "" {
			old.NDK = v.NDK
		}
		if v.BuildTools != "" {
			old.BuildTools = v.BuildTools
		}
		if v.CompileSdk != "" {
			old.CompileSdk = v.CompileSdk
		}
		if v.Gradle != "" {
			old.Gradle = v.Gradle
		}
		saved[k] = old
	}
	return writeJSON(path, saved)
}

func (s *Store) AppendLog(entries []LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(s.dir, "log.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// Log returns the newest entries first, at most limit of them.
func (s *Store) Log(limit int) []LogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Open(filepath.Join(s.dir, "log.jsonl"))
	if err != nil {
		return []LogEntry{}
	}
	defer f.Close()
	var all []LogEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e LogEntry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			all = append(all, e)
		}
	}
	out := []LogEntry{}
	for i := len(all) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, all[i])
	}
	return out
}

func readJSON(path string, v any) {
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, v)
	}
}

// writeJSON writes through a temp file so a crash never leaves half a file.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
