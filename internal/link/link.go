package link

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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

func IsSymlinkTo(link, want string) (bool, error) {
	got, err := os.Readlink(link)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return got == want, nil
}

func ReplaceSymlinkWithCopy(path string) error {
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}
	tmp := path + ".ccs-tmp"
	if err := copyTree(target, tmp); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	if err := os.Remove(path); err != nil {
		os.RemoveAll(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func ReplaceCopyWithSymlink(path, target string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.Symlink(target, path)
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	return copyFile(src, dst, info.Mode())
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
