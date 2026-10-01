// Package layout names the files ccs keeps under ~/.ccs and the Claude Code
// directories it works with.
package layout

import (
	"os"
	"path/filepath"
)

type Paths struct{ home string }

func New(home string) Paths { return Paths{home: home} }

func FromEnv() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return New(home), nil
}

func (p Paths) Root() string        { return filepath.Join(p.home, ".ccs") }
func (p Paths) ActiveFile() string  { return filepath.Join(p.Root(), "state", "active") }
func (p Paths) ProfilesDir() string { return filepath.Join(p.Root(), "profiles") }
func (p Paths) AccountsDir() string { return filepath.Join(p.Root(), "accounts") }
func (p Paths) RunDir() string      { return filepath.Join(p.Root(), "run") }
func (p Paths) BinDir() string      { return filepath.Join(p.Root(), "bin") }

// ProfileFile is the TOML file that defines profile name.
func (p Paths) ProfileFile(name string) string {
	return filepath.Join(p.ProfilesDir(), name+".toml")
}

// AccountDir is the CLAUDE_CONFIG_DIR of a profile with its own login.
func (p Paths) AccountDir(name string) string {
	return filepath.Join(p.AccountsDir(), name)
}

// SettingsFile is where the settings of profile name are written before
// they are passed to claude --settings.
func (p Paths) SettingsFile(name string) string {
	return filepath.Join(p.RunDir(), name+".settings.json")
}

// ShimPath is the claude shim written by `ccs init`.
func (p Paths) ShimPath() string { return filepath.Join(p.BinDir(), "claude") }

// ClaudeDir is the config directory Claude Code uses without
// CLAUDE_CONFIG_DIR; it is the default profile and the source every other
// profile links to.
func (p Paths) ClaudeDir() string { return filepath.Join(p.home, ".claude") }

// ClaudeJSON is the global state file of the default profile. Claude Code
// keeps it next to ~/.claude, not inside it, when CLAUDE_CONFIG_DIR is unset.
func (p Paths) ClaudeJSON() string { return filepath.Join(p.home, ".claude.json") }
