package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// EnsureSymlink makes link a symlink to target. An existing symlink is
// retargeted; an existing real file or directory is left alone and reported
// as an error.
func EnsureSymlink(target, link string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if existing, err := os.Readlink(link); err == nil {
		if existing == target {
			return nil
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		if info, statErr := os.Lstat(link); statErr == nil && info.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s exists and is not a symlink", link)
		}
	}
	return os.Symlink(target, link)
}

// ForceSymlink makes link a symlink to target, removing whatever currently
// exists at link, including real files and directories.
func ForceSymlink(target, link string) error {
	if existing, err := os.Readlink(link); err == nil && existing == target {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(link); err != nil {
		return err
	}
	return os.Symlink(target, link)
}

// IsEmpty reports whether path is missing, an empty file, or an empty
// directory. Shared placeholders created by `ccs init` are empty in this
// sense and may be replaced without asking.
func IsEmpty(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return info.Size() == 0, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}
