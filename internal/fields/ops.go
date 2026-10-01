package fields

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vika2603/ccs/internal/fsutil"
	"github.com/vika2603/ccs/internal/layout"
)

type LinkState int

const (
	Missing LinkState = iota
	Linked
	Forked
)

type Ops struct {
	paths    layout.Paths
	registry *Registry
}

func NewOps(p layout.Paths, r *Registry) Ops {
	return Ops{paths: p, registry: r}
}

func (o Ops) Fork(profile, field string) error {
	if _, ok := o.registry.lookupShared(field); !ok {
		return fmt.Errorf("field %q is not configured as shared", field)
	}
	profileDir := o.paths.ProfilePath(profile)
	linkPath := filepath.Join(profileDir, field)
	sharedPath := o.paths.SharedField(field)

	info, err := os.Lstat(linkPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%q is already a real copy; nothing to fork", linkPath)
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		return err
	}
	if target != sharedPath {
		return fmt.Errorf("symlink points to %q, not the expected shared path %q", target, sharedPath)
	}
	// Copy into a staging directory first so a failed copy leaves the
	// symlink in place instead of a missing or partial field.
	staging, err := os.MkdirTemp(profileDir, ".ccs-fork-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	staged := filepath.Join(staging, field)
	if err := fsutil.CopyTree(sharedPath, staged); err != nil {
		return err
	}
	if err := os.Remove(linkPath); err != nil {
		return err
	}
	if err := os.Rename(staged, linkPath); err != nil {
		return errors.Join(err, os.Symlink(sharedPath, linkPath))
	}
	return nil
}

func (o Ops) Share(profile, field string, onConflict ConflictFunc) error {
	if _, ok := o.registry.lookupShared(field); !ok {
		return fmt.Errorf("field %q is not configured as shared", field)
	}
	profileDir := o.paths.ProfilePath(profile)
	linkPath := filepath.Join(profileDir, field)
	sharedPath := o.paths.SharedField(field)

	info, err := os.Lstat(linkPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%q is already linked to shared; nothing to share", linkPath)
	}
	empty, err := fsutil.IsEmpty(sharedPath)
	if err != nil {
		return err
	}
	if !empty {
		overwrite, err := onConflict(field, sharedPath, linkPath)
		if err != nil {
			return err
		}
		if !overwrite {
			return errors.New("share aborted due to conflict")
		}
	}
	if err := os.RemoveAll(sharedPath); err != nil {
		return err
	}
	if err := fsutil.CopyTree(linkPath, sharedPath); err != nil {
		return err
	}
	if err := os.RemoveAll(linkPath); err != nil {
		return err
	}
	return fsutil.EnsureSymlink(sharedPath, linkPath)
}

func (o Ops) Relink(profile, field string) error {
	if _, ok := o.registry.lookupShared(field); !ok {
		return fmt.Errorf("field %q is not configured as shared", field)
	}
	profileDir := o.paths.ProfilePath(profile)
	linkPath := filepath.Join(profileDir, field)
	sharedPath := o.paths.SharedField(field)

	info, err := os.Lstat(linkPath)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		target, rerr := os.Readlink(linkPath)
		if rerr != nil {
			return rerr
		}
		if target == sharedPath {
			return nil
		}
		return fmt.Errorf("%q is a symlink to %q, not the expected shared path %q; resolve manually", linkPath, target, sharedPath)
	case err == nil:
		return fmt.Errorf("%q already exists as a real copy; run `ccs field share %s` first to push it into shared", linkPath, field)
	case errors.Is(err, os.ErrNotExist):
	default:
		return err
	}

	if _, sErr := os.Lstat(sharedPath); errors.Is(sErr, os.ErrNotExist) {
		if err := CreateSharedTargets(o.paths.SharedDir(), []Classification{{
			Name: field, Category: Shared, Kind: inferKind(field),
		}}); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return err
	}
	return fsutil.EnsureSymlink(sharedPath, linkPath)
}

func (o Ops) Status(profile string) (map[string]LinkState, error) {
	profileDir := o.paths.ProfilePath(profile)
	out := map[string]LinkState{}
	for _, c := range o.registry.Shared() {
		linkPath := filepath.Join(profileDir, c.Name)
		info, err := os.Lstat(linkPath)
		switch {
		case err != nil:
			out[c.Name] = Missing
		case info.Mode()&os.ModeSymlink != 0:
			out[c.Name] = Linked
		default:
			out[c.Name] = Forked
		}
	}
	return out, nil
}

func (r *Registry) lookupShared(name string) (Classification, bool) {
	c, ok := r.shared[name]
	return c, ok
}
