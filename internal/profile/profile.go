package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/link"
	"github.com/vika2603/ccs/internal/state"
)

type Manager struct {
	paths  layout.Paths
	fields *fields.Registry
	creds  creds.Store
}

func (m Manager) WithCreds(s creds.Store) Manager {
	m.creds = s
	return m
}

func (m Manager) Remove(name string) error {
	dir := m.paths.ProfilePath(name)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("profile %q does not exist", name)
		}
		return err
	}
	if m.creds != nil {
		if err := m.creds.Delete(dir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not delete keychain entry: %v\n", err)
		}
	}
	if err := os.Remove(m.paths.EnvFile(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "warning: could not remove env file %s: %v\n", m.paths.EnvFile(name), err)
	}
	return os.RemoveAll(dir)
}

func NewManager(p layout.Paths, r *fields.Registry) Manager {
	return Manager{paths: p, fields: r}
}

func (m Manager) Init() error {
	for _, d := range []string{m.paths.Root(), m.paths.StateDir(), m.paths.SharedDir(), m.paths.ProfilesDir(), m.paths.EnvDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if err := fields.CreateSharedTargets(m.paths.SharedDir(), m.fields.Shared()); err != nil {
		return err
	}
	return nil
}

func (m Manager) New(name string, blank bool) error {
	if err := state.ValidName(name); err != nil {
		return err
	}
	dir := m.paths.ProfilePath(name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if blank {
		return nil
	}
	if err := fields.CreateSharedTargets(m.paths.SharedDir(), m.fields.Shared()); err != nil {
		return err
	}
	for _, f := range m.fields.Shared() {
		sharedPath := m.paths.SharedField(f.Name)
		linkPath := filepath.Join(dir, f.Name)
		if err := link.EnsureSymlink(sharedPath, linkPath); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) List() ([]string, error) {
	entries, err := os.ReadDir(m.paths.ProfilesDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (m Manager) Path(name string) (string, error) {
	dir := m.paths.ProfilePath(name)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("profile %q does not exist", name)
		}
		return "", err
	}
	return dir, nil
}

func (m Manager) Rename(oldName, newName string) error {
	if err := state.ValidName(newName); err != nil {
		return err
	}
	oldDir := m.paths.ProfilePath(oldName)
	newDir := m.paths.ProfilePath(newName)
	if _, err := os.Stat(oldDir); err != nil {
		return fmt.Errorf("profile %q does not exist", oldName)
	}
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("profile %q already exists", newName)
	}
	if m.creds != nil {
		if err := creds.Migrate(m.creds, oldDir, newDir, filepath.Join(os.Getenv("HOME"), ".claude")); err != nil {
			return fmt.Errorf("migrate credentials: %w", err)
		}
	}
	if err := os.Rename(oldDir, newDir); err != nil {
		return err
	}
	oldEnv := m.paths.EnvFile(oldName)
	if _, err := os.Stat(oldEnv); err == nil {
		if err := os.MkdirAll(m.paths.EnvDir(), 0o755); err != nil {
			return err
		}
		if err := os.Rename(oldEnv, m.paths.EnvFile(newName)); err != nil {
			return fmt.Errorf("rename env file: %w", err)
		}
	}
	return nil
}

func (m Manager) Clone(source, dest string) error {
	if err := state.ValidName(source); err != nil {
		return err
	}
	if err := state.ValidName(dest); err != nil {
		return err
	}
	srcDir := m.paths.ProfilePath(source)
	if _, err := os.Stat(srcDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("profile %q does not exist", source)
		}
		return err
	}
	dstDir := m.paths.ProfilePath(dest)
	if _, err := os.Stat(dstDir); err == nil {
		return fmt.Errorf("profile %q already exists", dest)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dstDir)
		}
	}()
	if err := fields.CreateSharedTargets(m.paths.SharedDir(), m.fields.Shared()); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		class := m.fields.Describe(name)
		srcPath := filepath.Join(srcDir, name)
		dstPath := filepath.Join(dstDir, name)
		if class.Category == fields.Shared {
			info, lerr := os.Lstat(srcPath)
			if lerr != nil {
				return lerr
			}
			if info.Mode()&os.ModeSymlink != 0 {
				sharedPath := m.paths.SharedField(name)
				if err := link.EnsureSymlink(sharedPath, dstPath); err != nil {
					return err
				}
			} else {
				if err := fields.CopyByKind(srcPath, dstPath, class.Kind); err != nil {
					return err
				}
			}
		} else {
			if err := fields.CopyByKind(srcPath, dstPath, class.Kind); err != nil {
				return err
			}
		}
	}
	if m.creds != nil {
		data, err := m.creds.Read(srcDir)
		if err == nil {
			if werr := m.creds.Write(dstDir, data); werr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not write credentials for cloned profile: %v\n", werr)
			}
		}
	}
	srcEnv := m.paths.EnvFile(source)
	if _, err := os.Stat(srcEnv); err == nil {
		if err := os.MkdirAll(m.paths.EnvDir(), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(srcEnv)
		if err != nil {
			return fmt.Errorf("read source env file: %w", err)
		}
		if err := os.WriteFile(m.paths.EnvFile(dest), data, 0o600); err != nil {
			return fmt.Errorf("write cloned env file: %w", err)
		}
	}
	ok = true
	return nil
}

func (m Manager) Exists(name string) (bool, error) {
	_, err := os.Stat(m.paths.ProfilePath(name))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
