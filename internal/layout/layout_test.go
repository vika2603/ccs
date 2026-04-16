package layout

import (
	"path/filepath"
	"testing"
)

func TestPathsFromHome(t *testing.T) {
	home := "/tmp/fakehome"
	p := New(home)
	want := map[string]string{
		"Root":        filepath.Join(home, ".ccs"),
		"ConfigFile":  filepath.Join(home, ".ccs", "config.toml"),
		"StateDir":    filepath.Join(home, ".ccs", "state"),
		"ActiveFile":  filepath.Join(home, ".ccs", "state", "active"),
		"SharedDir":   filepath.Join(home, ".ccs", "shared"),
		"ProfilesDir": filepath.Join(home, ".ccs", "profiles"),
	}
	got := map[string]string{
		"Root":        p.Root(),
		"ConfigFile":  p.ConfigFile(),
		"StateDir":    p.StateDir(),
		"ActiveFile":  p.ActiveFile(),
		"SharedDir":   p.SharedDir(),
		"ProfilesDir": p.ProfilesDir(),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestProfilePath(t *testing.T) {
	p := New("/tmp/h")
	if got := p.ProfilePath("work"); got != "/tmp/h/.ccs/profiles/work" {
		t.Errorf("got %q", got)
	}
}
