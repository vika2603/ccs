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
func (p Paths) ConfigFile() string  { return filepath.Join(p.Root(), "config.toml") }
func (p Paths) StateDir() string    { return filepath.Join(p.Root(), "state") }
func (p Paths) ActiveFile() string  { return filepath.Join(p.StateDir(), "active") }
func (p Paths) SharedDir() string   { return filepath.Join(p.Root(), "shared") }
func (p Paths) ProfilesDir() string { return filepath.Join(p.Root(), "profiles") }
func (p Paths) EnvDir() string      { return filepath.Join(p.Root(), "env") }

func (p Paths) ProfilePath(name string) string {
	return filepath.Join(p.ProfilesDir(), name)
}

func (p Paths) EnvFile(name string) string {
	return filepath.Join(p.EnvDir(), name+".toml")
}

func (p Paths) SharedField(field string) string {
	return filepath.Join(p.SharedDir(), field)
}
