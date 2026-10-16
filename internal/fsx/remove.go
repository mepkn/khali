package fsx

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Remove deletes path. Read-only folders (Go's module cache, some npm
// packages) are made writable first so the delete doesn't stop half way.
func Remove(path string) error {
	if err := os.RemoveAll(path); err == nil {
		return nil
	}
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
	return os.RemoveAll(path)
}

// Trash moves path into ~/.Trash under a unique name. It is a rename on the
// same volume, so it is instant, but frees no space until the Trash is emptied.
func Trash(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	trash := filepath.Join(home, ".Trash")
	base := filepath.Base(path)
	// "node_modules" alone says nothing in the Trash; prefix the parent folder.
	parent := filepath.Base(filepath.Dir(path))
	name := parent + " " + base
	dest := filepath.Join(trash, name)
	for i := 2; Exists(dest); i++ {
		ext := filepath.Ext(name)
		dest = filepath.Join(trash, fmt.Sprintf("%s %s%s", strings.TrimSuffix(name, ext), time.Now().Format("15.04.05")+fmt.Sprint(i), ext))
	}
	return os.Rename(path, dest)
}
