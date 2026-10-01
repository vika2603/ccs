package layout

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/vika2603/ccs/internal/fsutil"
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

// Active returns the active profile name, or "" when none is set.
func (p Paths) Active() (string, error) { return readActive(p.ActiveFile()) }

// SetActive records name as the active profile. The file is replaced
// atomically, so a concurrent launch reads either the old or the new name.
func (p Paths) SetActive(name string) error {
	if err := ValidName(name); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(p.ActiveFile(), []byte(name+"\n"), 0o644)
}

// ClearActive unsets the active profile, which makes default active.
func (p Paths) ClearActive() error {
	if err := os.Remove(p.ActiveFile()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
