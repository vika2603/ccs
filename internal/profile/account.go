package profile

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"

	"github.com/vika2603/ccs/internal/layout"
)

// Account describes what pr talks to, for display only: "api: <host>" when
// its settings (or, for profiles without their own settings,
// ~/.claude/settings.json) set ANTHROPIC_BASE_URL, otherwise the OAuth
// account recorded in the profile's global state. It returns "" when neither
// is known. Tokens are never read.
func Account(p layout.Paths, pr Profile) string {
	if host := baseURLHost(pr.Settings); host != "" {
		return "api: " + host
	}
	var shared struct {
		Env map[string]any `json:"env"`
	}
	if readJSON(filepath.Join(p.ClaudeDir(), "settings.json"), &shared) == nil {
		if host := baseURLHost(map[string]any{"env": shared.Env}); host != "" {
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
