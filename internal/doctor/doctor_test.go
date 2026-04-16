package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
)

type fakeKeychain struct{ services []string }

func (f fakeKeychain) List() ([]string, error) { return f.services, nil }

func setup(t *testing.T, kc KeychainLister) (Checker, layout.Paths) {
	home := t.TempDir()
	p := layout.New(home)
	os.MkdirAll(p.SharedDir(), 0o755)
	os.MkdirAll(p.ProfilesDir(), 0o755)
	configured := fields.NewRegistry(config.Default())
	defaults := fields.NewRegistry(config.Default())
	return NewChecker(p, configured, defaults, kc, "/nonexistent/default/claude"), p
}

func TestCleanTree(t *testing.T) {
	c, _ := setup(t, fakeKeychain{})
	findings, err := c.Check()
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected clean, got: %v", findings)
	}
}

func TestDetectsBrokenSymlink(t *testing.T) {
	c, p := setup(t, fakeKeychain{})
	profile := p.ProfilePath("work")
	os.MkdirAll(profile, 0o755)
	os.Symlink(filepath.Join(p.SharedDir(), "skills"), filepath.Join(profile, "skills"))
	findings, _ := c.Check()
	if !containsKind(findings, BrokenSymlink) {
		t.Errorf("expected BrokenSymlink finding: %v", findings)
	}
}

func TestDetectsOrphanSharedField(t *testing.T) {
	c, p := setup(t, fakeKeychain{})
	os.MkdirAll(filepath.Join(p.SharedDir(), "orphan"), 0o755)
	findings, _ := c.Check()
	found := false
	for _, f := range findings {
		if f.Kind == OrphanSharedField && f.Detail == "orphan" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected OrphanSharedField finding: %v", findings)
	}
}

func TestDetectsOrphanKeychainEntry(t *testing.T) {
	bogus := "Claude Code-credentials-abcdef12"
	c, _ := setup(t, fakeKeychain{services: []string{
		"Claude Code-credentials",
		bogus,
	}})
	findings, _ := c.Check()
	found := false
	for _, f := range findings {
		if f.Kind == OrphanKeychainEntry && f.Detail == bogus {
			found = true
		}
	}
	if !found {
		t.Errorf("expected OrphanKeychainEntry for %q, got: %v", bogus, findings)
	}
}

func TestDetectsClassificationDrift(t *testing.T) {
	home := t.TempDir()
	p := layout.New(home)
	os.MkdirAll(p.SharedDir(), 0o755)
	os.MkdirAll(p.ProfilesDir(), 0o755)
	configured := fields.NewRegistry(config.Config{Isolated: []string{"skills"}})
	defaults := fields.NewRegistry(config.Config{Shared: []string{"skills"}})
	c := NewChecker(p, configured, defaults, fakeKeychain{}, "/nonexistent/default/claude")
	findings, _ := c.Check()
	found := false
	for _, f := range findings {
		if f.Kind == ClassificationDrift && f.Detail == "skills" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected ClassificationDrift for 'skills', got: %v", findings)
	}
}

func TestUserAdditionsDoNotDrift(t *testing.T) {
	home := t.TempDir()
	p := layout.New(home)
	os.MkdirAll(p.SharedDir(), 0o755)
	os.MkdirAll(p.ProfilesDir(), 0o755)
	configured := fields.NewRegistry(config.Config{
		Shared:   []string{"skills", "statusline.sh", "file-history"},
		Isolated: []string{"session-env"},
	})
	defaults := fields.NewRegistry(config.Config{
		Shared: []string{"skills"},
	})
	c := NewChecker(p, configured, defaults, fakeKeychain{}, "/nonexistent/default/claude")
	findings, _ := c.Check()
	for _, f := range findings {
		if f.Kind == ClassificationDrift {
			t.Errorf("user-added field %q should not be flagged as drift", f.Detail)
		}
	}
}

func containsKind(findings []Finding, k Kind) bool {
	for _, f := range findings {
		if f.Kind == k {
			return true
		}
	}
	return false
}
