package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/profile"
)

type memStore map[string][]byte

func (m memStore) Read(p string) ([]byte, error) {
	if b, ok := m[p]; ok {
		return b, nil
	}
	return nil, creds.ErrNotFound
}
func (m memStore) Write(p string, b []byte) error { m[p] = b; return nil }
func (m memStore) Delete(p string) error          { delete(m, p); return nil }

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func link(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}

func TestMigrate(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := t.TempDir()
	root := filepath.Join(home, ".ccs")
	shared := filepath.Join(root, "shared")
	claude := filepath.Join(home, ".claude")

	write(t, filepath.Join(root, "config.toml"), `version = 2
shared = ["skills", "rules", "CLAUDE.md", "settings.json"]
`)
	write(t, filepath.Join(shared, "skills", "a", "SKILL.md"), "a")
	write(t, filepath.Join(shared, "skills", "b", "SKILL.md"), "shared b")
	write(t, filepath.Join(shared, "rules", "r.md"), "r")
	write(t, filepath.Join(shared, "CLAUDE.md"), "old")
	write(t, filepath.Join(shared, "settings.json"), `{"model":"opus","env":{"X":"1"}}`)
	write(t, filepath.Join(claude, "skills", "b", "SKILL.md"), "mine b")
	write(t, filepath.Join(claude, "CLAUDE.md"), "new")
	write(t, filepath.Join(claude, "settings.json"), `{"model":"opus","env":{"X":"1"}}`)
	write(t, filepath.Join(root, "state", "active"), "gw\n")

	// gw: settings fork that only changes env and a scalar, plus an env file.
	gw := filepath.Join(root, "profiles", "gw")
	write(t, filepath.Join(gw, "history.jsonl"), "h")
	write(t, filepath.Join(gw, "settings.json"), `{"model":"sonnet","env":{"X":"1","BASE":"b"}}`)
	link(t, filepath.Join(shared, "skills"), filepath.Join(gw, "skills"))
	write(t, filepath.Join(root, "env", "gw.toml"), "[env]\nTOKEN = \"t\"\nBASE = \"env\"\n")
	// keep: settings fork with a different list stays isolated.
	keep := filepath.Join(root, "profiles", "keep")
	write(t, filepath.Join(keep, "settings.json"), `{"model":"opus","permissions":{"allow":["x"]}}`)
	link(t, filepath.Join(shared, "settings.json"), filepath.Join(root, "profiles", "keep", "CLAUDE.md"))

	store := memStore{gw: []byte("token")}
	if err := migrate(home, true, store); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gw); err != nil {
		t.Fatalf("dry run changed the tree: %v", err)
	}
	if err := migrate(home, false, store); err != nil {
		t.Fatal(err)
	}

	p := layout.New(home)
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := read(filepath.Join(claude, "skills", "a", "SKILL.md")); got != "a" {
		t.Errorf("missing skill not copied: %q", got)
	}
	if got := read(filepath.Join(claude, "skills", "b", "SKILL.md")); got != "mine b" {
		t.Errorf("existing skill replaced: %q", got)
	}
	if got := read(filepath.Join(claude, "CLAUDE.md")); got != "new" {
		t.Errorf("existing CLAUDE.md replaced: %q", got)
	}
	if got := read(filepath.Join(claude, "rules", "r.md")); got != "r" {
		t.Errorf("rules not copied: %q", got)
	}

	acct := p.AccountDir("gw")
	if got := read(filepath.Join(acct, "history.jsonl")); got != "h" {
		t.Errorf("history not moved: %q", got)
	}
	for _, name := range []string{"skills", "settings.json"} {
		if _, err := os.Lstat(filepath.Join(acct, name)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed from the account dir: %v", name, err)
		}
	}
	if store[acct] == nil || store[gw] != nil {
		t.Errorf("login not moved: %v", store)
	}
	pr, err := profile.Load(p, "gw")
	if err != nil {
		t.Fatal(err)
	}
	env, _ := pr.Settings["env"].(map[string]any)
	if !pr.Login || len(pr.Isolate) != 0 || pr.Settings["model"] != "sonnet" || env["TOKEN"] != "t" || env["BASE"] != "b" || env["X"] != nil {
		t.Errorf("gw profile: %+v", pr)
	}

	pr, err = profile.Load(p, "keep")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(pr.Isolate, ",") != "settings.json" {
		t.Errorf("keep should isolate settings.json: %+v", pr)
	}
	if !strings.Contains(read(filepath.Join(p.AccountDir("keep"), "settings.json")), `"x"`) {
		t.Error("keep's settings.json should stay in its account dir")
	}

	for _, rel := range []string{"config.toml", "shared", "env"} {
		if _, err := os.Stat(filepath.Join(root, rel)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed: %v", rel, err)
		}
	}
	if name, _ := p.Active(); name != "gw" {
		t.Errorf("active profile lost: %q", name)
	}
	if err := profile.Sync(p, pr); err != nil {
		t.Errorf("sync after migration: %v", err)
	}
	backups, _ := filepath.Glob(filepath.Join(home, ".ccs-v1-backup-*", "ccs", "shared", "CLAUDE.md"))
	if len(backups) != 1 || read(backups[0]) != "old" {
		t.Errorf("backup missing the shared CLAUDE.md: %v", backups)
	}
	if err := migrate(home, true, store); err == nil {
		t.Error("a migrated tree should be refused")
	}
}
