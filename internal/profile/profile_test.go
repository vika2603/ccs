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
	writeFile(t, filepath.Join(claude, "policy-limits.json"), "{}")

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
	for _, name := range []string{".credentials.json", "backups", "policy-limits.json"} {
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

	// Links to entries removed from ~/.claude are dropped; the user's own
	// links are kept even when dangling.
	if err := os.RemoveAll(filepath.Join(claude, "rules")); err != nil {
		t.Fatal(err)
	}
	own := filepath.Join(acct, "own-link")
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), own); err != nil {
		t.Fatal(err)
	}
	if err := Sync(p, pr); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(acct, "rules")); !os.IsNotExist(err) {
		t.Errorf("link to a removed ~/.claude entry should be dropped: %v", err)
	}
	if _, err := os.Lstat(own); err != nil {
		t.Errorf("the user's own link must be kept: %v", err)
	}
}

func TestSyncUnlinksIdentityEntries(t *testing.T) {
	p := layout.New(t.TempDir())
	writeFile(t, filepath.Join(p.ClaudeDir(), "remote-settings.json"), "{}")
	acct := p.AccountDir("work")
	if err := os.MkdirAll(acct, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(acct, "remote-settings.json")
	if err := os.Symlink(filepath.Join(p.ClaudeDir(), "remote-settings.json"), link); err != nil {
		t.Fatal(err)
	}
	if err := Sync(p, Profile{Name: "work", Login: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Errorf("link to an identity entry should be removed: %v", err)
	}
}

// A failed isolate copy must leave nothing at the destination that a later
// sync would keep as the account's own, partial data.
func TestSyncIsolateCopyIsAllOrNothing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads unreadable files")
	}
	p := layout.New(t.TempDir())
	writeFile(t, filepath.Join(p.ClaudeDir(), "projects", "a.txt"), "a")
	locked := filepath.Join(p.ClaudeDir(), "projects", "z.txt")
	writeFile(t, locked, "z")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	pr := Profile{Name: "work", Login: true, Isolate: []string{"projects"}}
	if err := Sync(p, pr); err == nil {
		t.Fatal("copy of an unreadable file should fail")
	}
	if _, err := os.Lstat(filepath.Join(p.AccountDir("work"), "projects")); !os.IsNotExist(err) {
		t.Fatalf("failed copy left a destination: %v", err)
	}
	if err := os.Chmod(locked, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Sync(p, pr); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(p.AccountDir("work"), "projects", "z.txt")); err != nil || string(b) != "z" {
		t.Errorf("retried copy incomplete: %q %v", b, err)
	}
}

func TestConcurrentSyncs(t *testing.T) {
	p := layout.New(t.TempDir())
	for _, name := range []string{"a", "b", "c", "d"} {
		writeFile(t, filepath.Join(p.ClaudeDir(), name, "f"), name)
	}
	pr := Profile{Name: "work", Login: true, Isolate: []string{"a", "b"}}
	errs := make(chan error, 8)
	for range cap(errs) {
		go func() { errs <- Sync(p, pr) }()
	}
	for range cap(errs) {
		if err := <-errs; err != nil {
			t.Errorf("concurrent sync: %v", err)
		}
	}
	for _, name := range []string{"a", "b", "c", "d"} {
		if b, err := os.ReadFile(filepath.Join(p.AccountDir("work"), name, "f")); err != nil || string(b) != name {
			t.Errorf("%s: %q %v", name, b, err)
		}
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
	if err := Remove(p, "ghost", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("removing a missing profile: %v", err)
	}
}

// Remove must not depend on the file's content: an unparsable profile, or one
// edited to drop login, still takes its account directory and login with it.
func TestRemoveIgnoresProfileContent(t *testing.T) {
	for name, content := range map[string]string{"broken": "login = tru", "flipped": "login = false\n"} {
		p := layout.New(t.TempDir())
		if err := Create(p, name, true); err != nil {
			t.Fatal(err)
		}
		writeFile(t, p.ProfileFile(name), content)
		var deleted string
		if err := Remove(p, name, func(dir string) error { deleted = dir; return nil }); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if deleted != p.AccountDir(name) {
			t.Errorf("%s: stored login not deleted", name)
		}
		for _, path := range []string{p.ProfileFile(name), p.AccountDir(name)} {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("%s: %s should be gone: %v", name, path, err)
			}
		}
	}
}

func TestAccount(t *testing.T) {
	p := layout.New(t.TempDir())
	writeFile(t, p.ClaudeJSON(), `{"oauthAccount":{"emailAddress":"me@x.com","organizationName":"me@x.com's Organization"}}`)
	writeFile(t, filepath.Join(p.AccountDir("work"), ".claude.json"), `{"oauthAccount":{"emailAddress":"w@x.com","organizationName":"Acme"}}`)
	writeFile(t, filepath.Join(p.AccountDir("own"), "settings.json"), `{"env":{"ANTHROPIC_BASE_URL":"https://own.example.com"}}`)

	cases := map[string]struct {
		pr   Profile
		want string
	}{
		"default":           {Profile{Name: "default"}, "me@x.com"},
		"login":             {Profile{Name: "work", Login: true}, "w@x.com (Acme)"},
		"gateway":           {Profile{Name: "gw", Settings: map[string]any{"env": map[string]any{"ANTHROPIC_BASE_URL": "https://gw.example.com/v1"}}}, "api: gw.example.com"},
		"no login":          {Profile{Name: "fresh", Login: true}, ""},
		"own settings.json": {Profile{Name: "own", Login: true}, "api: own.example.com"},
	}
	for name, tc := range cases {
		if got := Account(p, tc.pr); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}
