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

func (m Manager) Remove(name string, withKeychain bool) error {
	dir := m.paths.ProfilePath(name)
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("profile %q does not exist", name)
		}
		return err
	}
	if withKeychain && m.creds != nil {
		if err := m.creds.Delete(dir); err != nil {
			return fmt.Errorf("delete credentials: %w", err)
		}
	}
	return os.RemoveAll(dir)
}

func NewManager(p layout.Paths, r *fields.Registry) Manager {
	return Manager{paths: p, fields: r}
}

func (m Manager) Init() error {
	for _, d := range []string{m.paths.Root(), m.paths.StateDir(), m.paths.SharedDir(), m.paths.ProfilesDir()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if err := fields.CreateSharedTargets(m.paths.SharedDir(), m.fields.Shared()); err != nil {
		return err
	}
	return nil
}

func (m Manager) New(name string) error {
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
	return os.Rename(oldDir, newDir)
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
