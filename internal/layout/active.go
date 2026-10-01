package layout

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

var nameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// DefaultProfile names ~/.claude itself.
const DefaultProfile = "default"

// reserved holds names that would be shadowed by `ccs <name>` subcommands
// or that name the default profile.
var reserved = map[string]struct{}{
	DefaultProfile: {},
	"init":         {},
	"new":          {},
	"edit":         {},
	"ls":           {},
	"use":          {},
	"rm":           {},
	"help":         {},
	"completion":   {},
}

func ValidName(name string) error {
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: must match [A-Za-z0-9][A-Za-z0-9._-]{0,63}", name)
	}
	if _, ok := reserved[strings.ToLower(name)]; ok {
		return fmt.Errorf("profile name %q is reserved", name)
	}
	return nil
}

func readActive(path string) (string, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func acquire(lockPath string) (*flock.Flock, error) {
	lock := flock.New(lockPath)
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		ok, err := lock.TryLockContext(ctx, 100*time.Millisecond)
		cancel()
		if err != nil {
			return nil, err
		}
		if ok {
			return lock, nil
		}
	}
	return nil, fmt.Errorf("timed out acquiring state lock %s", lockPath)
}

func writeActive(path, name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	return writeLocked(path, []byte(name+"\n"))
}

func clearActive(path string) error {
	return writeLocked(path, nil)
}

func writeLocked(path string, data []byte) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := acquire(path + ".lock")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, lock.Unlock()) }()
	return os.WriteFile(path, data, 0o644)
}

// Active returns the active profile name, or "" when none is set.
func (p Paths) Active() (string, error) { return readActive(p.ActiveFile()) }

// SetActive records name as the active profile.
func (p Paths) SetActive(name string) error { return writeActive(p.ActiveFile(), name) }

// ClearActive unsets the active profile.
func (p Paths) ClearActive() error { return clearActive(p.ActiveFile()) }
