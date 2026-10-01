package profile

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"

	"github.com/vika2603/ccs/internal/profileenv"
)

// Account describes which account or endpoint a profile talks to, for
// display only: "email (organization)" for OAuth logins, otherwise
// "api: <host>" taken from ANTHROPIC_BASE_URL in the profile's env file or
// settings.json. It returns "" when neither is known. Tokens are never read.
func (m Manager) Account(name string) string {
	dir := m.paths.ProfilePath(name)
	var identity struct {
		OAuthAccount struct {
			EmailAddress     string `json:"emailAddress"`
			OrganizationName string `json:"organizationName"`
		} `json:"oauthAccount"`
	}
	if readJSON(filepath.Join(dir, ".claude.json"), &identity) == nil && identity.OAuthAccount.EmailAddress != "" {
		// Personal accounts get a default organization named after the email,
		// which adds nothing to the label.
		if org := identity.OAuthAccount.OrganizationName; org != "" && org != identity.OAuthAccount.EmailAddress+"'s Organization" {
			return identity.OAuthAccount.EmailAddress + " (" + org + ")"
		}
		return identity.OAuthAccount.EmailAddress
	}

	baseURL := ""
	if f, err := profileenv.Load(m.paths.EnvFile(name)); err == nil {
		baseURL = f.Env["ANTHROPIC_BASE_URL"]
	}
	if baseURL == "" {
		var settings struct {
			Env map[string]string `json:"env"`
		}
		if readJSON(filepath.Join(dir, "settings.json"), &settings) == nil {
			baseURL = settings.Env["ANTHROPIC_BASE_URL"]
		}
	}
	if baseURL == "" {
		return ""
	}
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		return "api: " + u.Host
	}
	return "api: " + baseURL
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
