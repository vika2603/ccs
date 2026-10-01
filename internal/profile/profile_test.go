package profile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vika2603/ccs/internal/layout"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCreateLoadAndTemplateRoundTrip(t *testing.T) {
	p := layout.New(t.TempDir())
	if err := Create(p, "work", true); err != nil {
		t.Fatal(err)
	}
	if err := Create(p, "work", false); err == nil {
		t.Error("creating an existing profile should fail")
	}
	pr, err := Load(p, "work")
	if err != nil {
		t.Fatal(err)
	}
	if !pr.Login || pr.Name != "work" {
		t.Errorf("unexpected profile %+v", pr)
	}
	if info, err := os.Stat(p.ProfileFile("work")); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("profile file must be private: %v %v", info.Mode(), err)
	}
	if _, err := os.Stat(p.AccountDir("work")); err != nil {
		t.Errorf("login profile should get an account directory: %v", err)
	}
	if _, err := Load(p, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing profile: %v", err)
	}
	if names, _ := List(p); len(names) != 1 || names[0] != "work" {
		t.Errorf("List = %v", names)
	}
}

func TestParseValidates(t *testing.T) {
	for name, content := range map[string]string{
		"unknown key":         "logn = true\n",
		"isolate needs login": "isolate = [\"projects\"]\n",
		"nested isolate":      "login = true\nisolate = [\"a/b\"]\n",
	} {
		if _, err := Parse([]byte(content)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	pr, err := Parse([]byte("[settings]\nmodel = \"x\"\n[settings.env]\nANTHROPIC_BASE_URL = \"https://gw\"\n"))
	if err != nil {
		t.Fatalf("arbitrary settings keys must pass through: %v", err)
	}
	if pr.Settings["model"] != "x" {
		t.Errorf("settings = %v", pr.Settings)
	}
}

func TestSyncLinksSharesAndIsolates(t *testing.T) {
	p := layout.New(t.TempDir())
	claude := p.ClaudeDir()
	writeFile(t, filepath.Join(claude, "skills", "a", "SKILL.md"), "a")
	writeFile(t, filepath.Join(claude, "settings.json"), "{}")
	writeFile(t, filepath.Join(claude, "history.jsonl"), "shared-history")
	writeFile(t, filepath.Join(claude, ".credentials.json"), "secret")
	writeFile(t, filepath.Join(claude, "backups", "b.json"), "{}")

	pr := Profile{Name: "work", Login: true, Isolate: []string{"history.jsonl"}}
	acct := p.AccountDir("work")
	writeFile(t, filepath.Join(acct, "projects", "own.txt"), "mine")
	if err := Sync(p, pr); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"skills", "settings.json"} {
		if target, err := os.Readlink(filepath.Join(acct, name)); err != nil || target != filepath.Join(claude, name) {
			t.Errorf("%s should link to ~/.claude: %q %v", name, target, err)
		}
	}
	for _, name := range []string{".credentials.json", "backups"} {
		if _, err := os.Lstat(filepath.Join(acct, name)); err == nil {
			t.Errorf("identity entry %s must not be linked", name)
		}
	}
	if b, err := os.ReadFile(filepath.Join(acct, "projects", "own.txt")); err != nil || string(b) != "mine" {
		t.Errorf("existing account data must be kept: %q %v", b, err)
	}
	info, err := os.Lstat(filepath.Join(acct, "history.jsonl"))
	if err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("isolated entry should be a real copy: %v", err)
	}

	// Isolating an entry that was linked before replaces the link by a copy;
	// later writes stay in the account.
	pr.Isolate = append(pr.Isolate, "settings.json")
	if err := Sync(p, pr); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(filepath.Join(acct, "settings.json")); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("settings.json should now be a copy: %v", err)
	}
	writeFile(t, filepath.Join(acct, "settings.json"), `{"own":true}`)
	if b, _ := os.ReadFile(filepath.Join(claude, "settings.json")); string(b) != "{}" {
		t.Errorf("isolated write leaked into ~/.claude: %q", b)
	}

	// New ~/.claude entries are linked on the next sync.
	writeFile(t, filepath.Join(claude, "rules", "r.md"), "r")
	if err := Sync(p, pr); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Readlink(filepath.Join(acct, "rules")); err != nil {
		t.Errorf("new ~/.claude entry should be linked: %v", err)
	}
}

func TestWriteSettingsIsPrivateAndRemovedWhenEmpty(t *testing.T) {
	p := layout.New(t.TempDir())
	pr := Profile{Name: "gw", Settings: map[string]any{"env": map[string]any{"ANTHROPIC_AUTH_TOKEN": "t"}}}
	path, err := WriteSettings(p, pr)
	if err != nil || path == "" {
		t.Fatalf("WriteSettings: %q %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("settings file must be private: %v %v", info.Mode(), err)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), `"ANTHROPIC_AUTH_TOKEN": "t"`) {
		t.Errorf("settings JSON = %s", b)
	}
	pr.Settings = nil
	if path, err := WriteSettings(p, pr); err != nil || path != "" {
		t.Fatalf("empty settings: %q %v", path, err)
	}
	if _, err := os.Stat(p.SettingsFile("gw")); !os.IsNotExist(err) {
		t.Errorf("stale settings file should be removed: %v", err)
	}
}

func TestRemove(t *testing.T) {
	p := layout.New(t.TempDir())
	if err := Create(p, "work", true); err != nil {
		t.Fatal(err)
	}
	var deleted string
	if err := Remove(p, "work", func(dir string) error { deleted = dir; return nil }); err != nil {
		t.Fatal(err)
	}
	if deleted != p.AccountDir("work") {
		t.Errorf("stored login not deleted for %q", deleted)
	}
	for _, path := range []string{p.ProfileFile("work"), p.AccountDir("work")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s should be gone: %v", path, err)
		}
	}
	if err := Remove(p, layout.DefaultProfile, nil); err == nil {
		t.Error("the default profile must not be removable")
	}
}

func TestAccount(t *testing.T) {
	p := layout.New(t.TempDir())
	writeFile(t, p.ClaudeJSON(), `{"oauthAccount":{"emailAddress":"me@x.com","organizationName":"me@x.com's Organization"}}`)
	writeFile(t, filepath.Join(p.AccountDir("work"), ".claude.json"), `{"oauthAccount":{"emailAddress":"w@x.com","organizationName":"Acme"}}`)

	cases := map[string]struct {
		pr   Profile
		want string
	}{
		"default":  {Profile{Name: "default"}, "me@x.com"},
		"login":    {Profile{Name: "work", Login: true}, "w@x.com (Acme)"},
		"gateway":  {Profile{Name: "gw", Settings: map[string]any{"env": map[string]any{"ANTHROPIC_BASE_URL": "https://gw.example.com/v1"}}}, "api: gw.example.com"},
		"no login": {Profile{Name: "fresh", Login: true}, ""},
	}
	for name, tc := range cases {
		if got := Account(p, tc.pr); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}
