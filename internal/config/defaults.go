package config

// Default returns the shipped classification based on Claude Code 4.x
// on-disk layout. Shared entries are read by Claude Code each turn and
// safe to symlink across profiles. Isolated entries hold per-profile
// state Claude Code writes back to, including the account identity in
// .claude.json (oauthAccount, userID, migrationVersion, onboarding
// flags) that pairs with the OAuth token in the platform credential
// store. Export.Exclude lists entries that are intentionally dropped
// from `ccs export` — caches and stats that can be regenerated.
//
// .credentials.json stays in Isolated for Linux, where the file-backed
// credential store writes to <profile>/.credentials.json.
func Default() Config {
	return Config{
		Version: supportedVersion,
		Shared:  []string{"skills", "commands", "agents", "plugins", "CLAUDE.md", "settings.json"},
		Isolated: []string{
			".claude.json",
			".credentials.json",
			"backups",
			"history.jsonl",
			"ide",
			"mcp-needs-auth-cache.json",
			"policy-limits.json",
			"projects",
			"sessions",
			"shell-snapshots",
			"statsig",
			"todos",
		},
		Export: Export{
			Exclude: []string{"cache", "chrome", "paste-cache", "stats-cache.json"},
		},
	}
}
