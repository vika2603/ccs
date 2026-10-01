// Package profile reads and applies ccs profiles.
//
// A profile is a TOML file in ~/.ccs/profiles. Without login it runs Claude
// Code on ~/.claude directly and only adds settings. With login it gets its
// own config directory for a separate OAuth login, in which every entry of
// ~/.claude is a symlink back to ~/.claude except the account identity and
// the entries the profile isolates.
package profile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/vika2603/ccs/internal/fsutil"
	"github.com/vika2603/ccs/internal/layout"
)

// identity lists config-directory entries that belong to one account and
// are never linked: the global state with the OAuth account, the Linux
// credentials file, and the backups Claude Code makes of the global state.
var identity = []string{".claude.json", ".credentials.json", "backups"}

// Profile is the content of ~/.ccs/profiles/<name>.toml.
type Profile struct {
	Name     string         `toml:"-"`
	Login    bool           `toml:"login"`
	Isolate  []string       `toml:"isolate"`
	Settings map[string]any `toml:"settings"`
}

var ErrNotFound = errors.New("profile not found")

// Load reads profile name. The default profile always exists and is empty.
func Load(p layout.Paths, name string) (Profile, error) {
	if name == layout.DefaultProfile {
		return Profile{Name: name}, nil
	}
	if err := layout.ValidName(name); err != nil {
		return Profile{}, err
	}
	b, err := os.ReadFile(p.ProfileFile(name))
	if errors.Is(err, os.ErrNotExist) {
		return Profile{}, fmt.Errorf("profile %q does not exist: %w", name, ErrNotFound)
	}
	if err != nil {
		return Profile{}, err
	}
	pr, err := Parse(b)
	if err != nil {
		return Profile{}, fmt.Errorf("%s: %w", p.ProfileFile(name), err)
	}
	pr.Name = name
	return pr, nil
}

// Parse decodes and validates a profile file.
func Parse(b []byte) (Profile, error) {
	var pr Profile
	md, err := toml.Decode(string(b), &pr)
	if err != nil {
		return Profile{}, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		for _, key := range undecoded {
			// Everything under [settings] is passed to Claude Code as is.
			if len(key) == 0 || key[0] != "settings" {
				return Profile{}, fmt.Errorf("unknown key %q", key.String())
			}
		}
	}
	if len(pr.Isolate) > 0 && !pr.Login {
		return Profile{}, errors.New("isolate requires login = true; without its own directory a profile shares all of ~/.claude")
	}
	for _, name := range pr.Isolate {
		if name == "" || strings.ContainsRune(name, '/') || name == "." || name == ".." {
			return Profile{}, fmt.Errorf("isolate entry %q must be a top-level name in ~/.claude", name)
		}
	}
	return pr, nil
}

// List returns profile names in sorted order, excluding the default profile.
func List(p layout.Paths) ([]string, error) {
	entries, err := os.ReadDir(p.ProfilesDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".toml"); ok && !e.IsDir() && layout.ValidName(name) == nil {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// Template returns the initial content of a new profile file.
func Template(name string, login bool) string {
	return fmt.Sprintf(`# ccs profile %[1]q. Edit with: ccs edit %[1]s
#
# login = true gives this profile its own Claude Code config directory
# (~/.ccs/accounts/%[1]s) for a separate OAuth login. Everything in ~/.claude
# is linked into it except the account identity and the names in isolate.
# login = false runs Claude Code on ~/.claude itself.
login = %[2]t

# Entries this profile keeps to itself instead of linking to ~/.claude
# (login = true only). An existing ~/.claude entry is copied on first use.
# isolate = ["projects", "history.jsonl"]

# Settings applied on top of ~/.claude/settings.json for this profile only,
# passed to claude --settings. For an API gateway, for example:
# [settings.env]
# ANTHROPIC_BASE_URL = "https://gateway.example.com"
# ANTHROPIC_AUTH_TOKEN = "..."
`, name, login)
}

// Create writes a new profile file from Template.
func Create(p layout.Paths, name string, login bool) error {
	if err := layout.ValidName(name); err != nil {
		return err
	}
	path := p.ProfileFile(name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("profile %q already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if login {
		if err := os.MkdirAll(p.AccountDir(name), 0o700); err != nil {
			return err
		}
	}
	return fsutil.WriteFileAtomic(path, []byte(Template(name, login)), 0o600)
}

// ConfigDir returns the CLAUDE_CONFIG_DIR for pr, or "" when it runs on
// ~/.claude.
func (pr Profile) ConfigDir(p layout.Paths) string {
	if !pr.Login {
		return ""
	}
	return p.AccountDir(pr.Name)
}

// Sync brings a login profile's directory in line with ~/.claude: every
// ~/.claude entry missing from it is linked, and isolated entries that are
// still links are replaced by a copy. Real files and directories already in
// the account directory are never touched, so nothing the account wrote is
// lost; to share such an entry again, delete it from the account directory.
func Sync(p layout.Paths, pr Profile) error {
	if !pr.Login {
		return nil
	}
	accountDir := p.AccountDir(pr.Name)
	if err := os.MkdirAll(accountDir, 0o700); err != nil {
		return err
	}
	entries, err := os.ReadDir(p.ClaudeDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if slices.Contains(identity, name) {
			continue
		}
		src := filepath.Join(p.ClaudeDir(), name)
		dst := filepath.Join(accountDir, name)
		current, err := os.Readlink(dst)
		isLink := err == nil
		exists := isLink
		if !isLink {
			if _, err := os.Lstat(dst); err == nil {
				exists = true
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		switch {
		case slices.Contains(pr.Isolate, name):
			if isLink && current == src {
				if err := os.Remove(dst); err != nil {
					return err
				}
				exists = false
			}
			if !exists {
				if err := fsutil.CopyTree(src, dst); err != nil {
					return fmt.Errorf("isolate %s: %w", name, err)
				}
			}
		case !exists:
			if err := os.Symlink(src, dst); err != nil {
				return err
			}
		}
	}
	return nil
}

// WriteSettings writes pr's settings as JSON for claude --settings and
// returns the file path, or "" when the profile has no settings. The file is
// private because settings usually carry API tokens, which must not appear
// on a command line.
func WriteSettings(p layout.Paths, pr Profile) (string, error) {
	path := p.SettingsFile(pr.Name)
	if len(pr.Settings) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		return "", nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(pr.Settings); err != nil {
		return "", err
	}
	if err := os.MkdirAll(p.RunDir(), 0o700); err != nil {
		return "", err
	}
	return path, fsutil.WriteFileAtomic(path, buf.Bytes(), 0o600)
}

// Remove deletes profile name: its file, its settings file, and for a login
// profile its account directory. deleteCreds removes the account's stored
// OAuth token and is called before the directory is deleted.
func Remove(p layout.Paths, name string, deleteCreds func(configDir string) error) error {
	if name == layout.DefaultProfile {
		return errors.New("the default profile is ~/.claude itself and cannot be removed")
	}
	pr, err := Load(p, name)
	if err != nil {
		return err
	}
	if pr.Login {
		if err := deleteCreds(p.AccountDir(name)); err != nil {
			return fmt.Errorf("delete stored login: %w", err)
		}
		if err := os.RemoveAll(p.AccountDir(name)); err != nil {
			return err
		}
	}
	if err := os.Remove(p.SettingsFile(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Remove(p.ProfileFile(name))
}
