// Package fsx measures and removes folders.
package fsx

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// Sizer adds up the disk blocks a folder really uses. A file with several
// hard links is counted once per Sizer, so share one Sizer across a scan.
type Sizer struct {
	mu   sync.Mutex
	seen map[[2]uint64]struct{}
}

func NewSizer() *Sizer {
	return &Sizer{seen: map[[2]uint64]struct{}{}}
}

// Usage is the allocated size of a path and how many entries it holds.
type Usage struct {
	Bytes int64
	Files int
}

// Size walks path without following symlinks. Unreadable entries are skipped.
func (s *Sizer) Size(path string) Usage {
	var u Usage
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			u.Bytes += info.Size()
			u.Files++
			return nil
		}
		if st.Nlink > 1 && !d.IsDir() {
			key := [2]uint64{uint64(st.Dev), st.Ino}
			s.mu.Lock()
			_, dup := s.seen[key]
			s.seen[key] = struct{}{}
			s.mu.Unlock()
			if dup {
				return nil
			}
		}
		u.Bytes += st.Blocks * 512
		u.Files++
		return nil
	})
	return u
}

// Exists reports whether path exists, without following a final symlink.
func Exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// IsDir reports whether path is a real directory (not a symlink to one).
func IsDir(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.IsDir()
}
