package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/profileenv"
)

func TestAccount(t *testing.T) {
	p := layout.New(t.TempDir())
	m := NewManager(p, fields.NewRegistry(config.Default()))
	write := func(name, file, content string) {
		t.Helper()
		dir := p.ProfilePath(name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("oauth", ".claude.json", `{"oauthAccount":{"emailAddress":"a@x.com","organizationName":"Acme"}}`)
	write("settings", "settings.json", `{"env":{"ANTHROPIC_BASE_URL":"https://gw.example.com/v1","ANTHROPIC_AUTH_TOKEN":"secret"}}`)
	write("envfile", ".claude.json", `{}`)
	if err := profileenv.Save(p.EnvFile("envfile"), profileenv.File{Env: map[string]string{"ANTHROPIC_BASE_URL": "https://api.other.dev"}}); err != nil {
		t.Fatal(err)
	}
	write("unknown", ".claude.json", `{}`)
	write("personal", ".claude.json", `{"oauthAccount":{"emailAddress":"me@x.com","organizationName":"me@x.com's Organization"}}`)

	for name, want := range map[string]string{
		"oauth":    "a@x.com (Acme)",
		"settings": "api: gw.example.com",
		"envfile":  "api: api.other.dev",
		"unknown":  "",
		"personal": "me@x.com",
	} {
		if got := m.Account(name); got != want {
			t.Errorf("Account(%q) = %q, want %q", name, got, want)
		}
	}
}
