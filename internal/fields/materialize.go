package fields

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
	info, statErr := os.Lstat(sharedPath)
	switch {
	case statErr == nil:
		empty, err := isSharedPlaceholderEmpty(sharedPath, info)
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
	case errors.Is(statErr, os.ErrNotExist):
		if err := moveOrCopy(srcPath, sharedPath, move); err != nil {
			return err
		}
	default:
		return statErr
	}
	return ensureSymlinkTo(sharedPath, linkPath)
}

func isSharedPlaceholderEmpty(path string, info os.FileInfo) (bool, error) {
	if !info.IsDir() {
		return info.Size() == 0, nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func ensureSymlinkTo(target, linkPath string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			existing, _ := os.Readlink(linkPath)
			if existing == target {
				return nil
			}
		}
		if err := os.RemoveAll(linkPath); err != nil {
			return err
		}
	}
	return os.Symlink(target, linkPath)
}

func moveOrCopy(src, dst string, move bool) error {
	if move {
		return os.Rename(src, dst)
	}
	return copyPath(src, dst)
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		children, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, c := range children {
			if err := copyPath(filepath.Join(src, c.Name()), filepath.Join(dst, c.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func SelectExportMaterial(profileDir string, reg *Registry, mode ExportMode) ([]Entry, error) {
	dirEntries, err := os.ReadDir(profileDir)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, e := range dirEntries {
		name := e.Name()
		if name == ".credentials.json" || name == ".claude.json" {
			continue
		}
		if reg.IsExcludedFromExport(name) {
			continue
		}
		class := reg.Describe(name)
		if class.Category == Isolated && mode != ExportFull {
			continue
		}
		path := filepath.Join(profileDir, name)
		kind, err := detectKind(path)
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{
			Name: name,
			Path: path,
			Kind: kind,
		})
	}
	return out, nil
}
