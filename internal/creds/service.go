// Package creds removes the OAuth login Claude Code stores for a config
// directory.
package creds

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
)

// serviceName is the macOS Keychain service under which Claude Code stores
// the login for configDir: the bare prefix for ~/.claude, and the prefix plus
// the first 8 hex digits of sha256(configDir) for any CLAUDE_CONFIG_DIR.
// Only the latter is needed, because ccs never deletes the ~/.claude login.
func serviceName(configDir string) (string, error) {
	abs, err := filepath.Abs(configDir)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(abs))
	return "Claude Code-credentials-" + hex.EncodeToString(sum[:])[:8], nil
}
