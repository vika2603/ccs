package profile

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"

	"github.com/vika2603/ccs/internal/layout"
)

// Account describes what pr talks to, for display only: "api: <host>" when
// its [settings] or the settings.json it runs with set ANTHROPIC_BASE_URL,
// otherwise the OAuth
// account recorded in the profile's global state. It returns "" when neither
// is known. Tokens are never read.
func Account(p layout.Paths, pr Profile) string {
	if host := baseURLHost(pr.Settings); host != "" {
		return "api: " + host
	}
	// A login profile may keep its own settings.json (isolate); otherwise it
	// is a link to ~/.claude/settings.json or not created yet.
	settingsPath := filepath.Join(p.ClaudeDir(), "settings.json")
	if dir := pr.ConfigDir(p); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "settings.json")); err == nil {
			settingsPath = filepath.Join(dir, "settings.json")
		}
	}
	var fileSettings map[string]any
	if readJSON(settingsPath, &fileSettings) == nil {
		if host := baseURLHost(fileSettings); host != "" {
			return "api: " + host
		}
	}

	statePath := p.ClaudeJSON()
	if pr.Login {
		statePath = filepath.Join(p.AccountDir(pr.Name), ".claude.json")
	}
	var state struct {
		OAuthAccount struct {
			EmailAddress     string `json:"emailAddress"`
			OrganizationName string `json:"organizationName"`
		} `json:"oauthAccount"`
	}
	if readJSON(statePath, &state) != nil || state.OAuthAccount.EmailAddress == "" {
		return ""
	}
	email, org := state.OAuthAccount.EmailAddress, state.OAuthAccount.OrganizationName
	// Personal accounts get a default organization named after the email,
	// which adds nothing to the label.
	if org == "" || org == email+"'s Organization" {
		return email
	}
	return email + " (" + org + ")"
}

func baseURLHost(settings map[string]any) string {
	env, _ := settings["env"].(map[string]any)
	raw, _ := env["ANTHROPIC_BASE_URL"].(string)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
