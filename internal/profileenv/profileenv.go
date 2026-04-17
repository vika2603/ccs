// Package profileenv manages per-profile environment variables: load and save
// a TOML file at ~/.ccs/env/<profile>.toml and produce shell-evaluable output
// that exports them when a profile is activated.
package profileenv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type File struct {
	Env map[string]string `toml:"env"`
}

var nameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidName(k string) error {
	if k == "" {
		return errors.New("env var name is empty")
	}
	if !nameRE.MatchString(k) {
		return fmt.Errorf("invalid env var name %q: must match [A-Za-z_][A-Za-z0-9_]*", k)
	}
	return nil
}

func Load(path string) (File, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return File{Env: map[string]string{}}, nil
	}
	if err != nil {
		return File{}, err
	}
	var f File
	if err := toml.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if f.Env == nil {
		f.Env = map[string]string{}
	}
	for k := range f.Env {
		if err := ValidName(k); err != nil {
			return File{}, fmt.Errorf("%s: %w", path, err)
		}
	}
	return f, nil
}

func Save(path string, f File) error {
	for k := range f.Env {
		if err := ValidName(k); err != nil {
			return err
		}
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".env.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := toml.NewEncoder(tmp).Encode(f); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// Keys returns env var names in deterministic (sorted) order.
func (f File) Keys() []string {
	keys := make([]string, 0, len(f.Env))
	for k := range f.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Signature returns a short identifier that changes when either the active
// profile or its env file on disk changes. An empty profile yields a fixed
// sentinel so the hook can detect "no active profile" -> "profile X" transitions.
func Signature(profile, envFilePath string) string {
	if profile == "" {
		return "!none"
	}
	if envFilePath == "" {
		return profile + ":0"
	}
	info, err := os.Stat(envFilePath)
	if err != nil {
		return profile + ":0"
	}
	return profile + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

// ParseAssignment splits a single "KEY=VALUE" argument. The VALUE part may
// itself contain '=' (only the first separator matters).
func ParseAssignment(arg string) (string, string, error) {
	k, v, ok := strings.Cut(arg, "=")
	if !ok {
		return "", "", fmt.Errorf("expected KEY=VALUE, got %q", arg)
	}
	if err := ValidName(k); err != nil {
		return "", "", err
	}
	return k, v, nil
}
