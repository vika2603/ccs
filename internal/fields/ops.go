package fields

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/link"
	"github.com/vika2603/ccs/internal/tui"
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
	kind, err := detectKind(sharedPath)
	if err != nil {
		return err
	}
	if err := os.Remove(linkPath); err != nil {
		return err
	}
	return copyByKind(sharedPath, linkPath, kind)
}

func (o Ops) Share(profile, field string, out io.Writer, in io.Reader) error {
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
	kind, err := detectKind(linkPath)
	if err != nil {
		return err
	}
	nonEmpty, err := sharedHasContent(sharedPath)
	if err != nil {
		return err
	}
	if nonEmpty {
		res, err := tui.PromptConflict(
			tui.Entry{Name: field, Path: sharedPath},
			tui.Entry{Name: field, Path: linkPath},
			out, in,
		)
		if err != nil {
			return err
		}
		if res != tui.ResolveOverwrite {
			return errors.New("share aborted due to conflict")
		}
		if err := os.RemoveAll(sharedPath); err != nil {
			return err
		}
	} else {
		_ = os.RemoveAll(sharedPath)
	}
	if err := copyByKind(linkPath, sharedPath, kind); err != nil {
		return err
	}
	if err := os.RemoveAll(linkPath); err != nil {
		return err
	}
	return link.EnsureSymlink(sharedPath, linkPath)
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
		return fmt.Errorf("%q already exists as a real copy; run `ccs share %s` first to push it into shared", linkPath, field)
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
	return link.EnsureSymlink(sharedPath, linkPath)
}

func (o Ops) RelinkAll(profile string) ([]string, error) {
	var relinked []string
	for _, c := range o.registry.Shared() {
		linkPath := filepath.Join(o.paths.ProfilePath(profile), c.Name)
		info, err := os.Lstat(linkPath)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			return relinked, err
		}
		if err := o.Relink(profile, c.Name); err != nil {
			return relinked, err
		}
		relinked = append(relinked, c.Name)
	}
	return relinked, nil
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

func sharedHasContent(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return false, err
		}
		return len(entries) > 0, nil
	}
	return info.Size() > 0, nil
}

func copyByKind(src, dst string, kind Kind) error {
	switch kind {
	case KindFile:
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return copyPath(src, dst)
	case KindDir:
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		children, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range children {
			if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown kind %v", kind)
	}
}
