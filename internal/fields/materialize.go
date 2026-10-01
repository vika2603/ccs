package fields

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vika2603/ccs/internal/fsutil"
)

// ConflictFunc decides whether incomingPath may overwrite the non-empty
// shared entry at existingPath.
type ConflictFunc func(name, existingPath, incomingPath string) (overwrite bool, err error)

type Prompter interface {
	OnSharedConflict(name, existingPath, incomingPath string) (overwrite bool, err error)
	OnUnknownEntry(name string)
}

func CreateSharedTargets(sharedDir string, entries []Classification) error {
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		return err
	}
	for _, e := range entries {
		if e.Category != Shared {
			continue
		}
		target := filepath.Join(sharedDir, e.Name)
		info, err := os.Lstat(target)
		switch {
		case err == nil:
			if e.Kind == KindDir && !info.IsDir() {
				return fmt.Errorf("shared target %q exists but is not a directory", target)
			}
			if e.Kind == KindFile && info.IsDir() {
				return fmt.Errorf("shared target %q exists but is a directory, not a file", target)
			}
			continue
		case errors.Is(err, os.ErrNotExist):
		default:
			return err
		}
		switch e.Kind {
		case KindDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case KindFile:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
	}
	return nil
}

func ImportEntries(srcProfileDir, dstProfileDir, sharedDir string, reg *Registry, prompter Prompter, move bool) error {
	entries, err := os.ReadDir(srcProfileDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if reg.IsUnknown(name) {
			prompter.OnUnknownEntry(name)
		}
		srcPath := filepath.Join(srcProfileDir, name)
		switch reg.Classify(name) {
		case Shared:
			if err := importSharedEntry(name, srcPath, sharedDir, dstProfileDir, prompter, move); err != nil {
				return err
			}
		case Isolated:
			if err := moveOrCopy(srcPath, filepath.Join(dstProfileDir, name), move); err != nil {
				return err
			}
		}
	}
	return nil
}

func importSharedEntry(name, srcPath, sharedDir, dstProfileDir string, prompter Prompter, move bool) error {
	sharedPath := filepath.Join(sharedDir, name)
	linkPath := filepath.Join(dstProfileDir, name)
	empty, err := fsutil.IsEmpty(sharedPath)
	if err != nil {
		return err
	}
	if !empty {
		overwrite, err := prompter.OnSharedConflict(name, sharedPath, srcPath)
		if err != nil {
			return err
		}
		if !overwrite {
			return fmt.Errorf("import aborted at shared entry %q", name)
		}
	}
	if err := os.RemoveAll(sharedPath); err != nil {
		return err
	}
	if err := moveOrCopy(srcPath, sharedPath, move); err != nil {
		return err
	}
	return fsutil.ForceSymlink(sharedPath, linkPath)
}

func moveOrCopy(src, dst string, move bool) error {
	if move {
		return os.Rename(src, dst)
	}
	return fsutil.CopyTree(src, dst)
}
