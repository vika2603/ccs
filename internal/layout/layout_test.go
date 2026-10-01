package layout

import (
	"path/filepath"
	"testing"
)

func TestPathsFromHome(t *testing.T) {
	home := "/tmp/fakehome"
	p := New(home)
	for got, want := range map[string]string{
		p.Root():               filepath.Join(home, ".ccs"),
		p.ActiveFile():         filepath.Join(home, ".ccs", "state", "active"),
		p.ProfileFile("work"):  filepath.Join(home, ".ccs", "profiles", "work.toml"),
		p.AccountDir("work"):   filepath.Join(home, ".ccs", "accounts", "work"),
		p.SettingsFile("work"): filepath.Join(home, ".ccs", "run", "work.settings.json"),
		p.ShimPath("claude"):   filepath.Join(home, ".ccs", "bin", "claude"),
		p.ClaudeDir():          filepath.Join(home, ".claude"),
		p.ClaudeJSON():         filepath.Join(home, ".claude.json"),
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestValidNameRejectsReserved(t *testing.T) {
	for _, name := range []string{"default", "Default", "use", "ls", "init"} {
		if err := ValidName(name); err == nil {
			t.Errorf("ValidName(%q) should fail", name)
		}
	}
	if err := ValidName("work"); err != nil {
		t.Errorf("ValidName(work): %v", err)
	}
}
