package state

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

var reserved = map[string]struct{}{
	"default": {},
	"shared":  {},
	"state":   {},
	"config":  {},
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

func Read(path string) (string, error) {
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
	for attempt := 0; attempt < 2; attempt++ {
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

func Write(path, name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := acquire(path + ".lock")
	if err != nil {
		return err
	}
	defer lock.Unlock()
	return os.WriteFile(path, []byte(name+"\n"), 0o644)
}

func Clear(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lock, err := acquire(path + ".lock")
	if err != nil {
		return err
	}
	defer lock.Unlock()
	return os.WriteFile(path, nil, 0o644)
}
