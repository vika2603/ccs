// Package launch builds the environment and resolves the binary used to exec
// claude.
package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProfileVar names the profile a claude process was launched with, so a
// claude started inside it can run with the same profile.
const ProfileVar = "CCS_PROFILE"

// Env returns env for launching profile name: CLAUDE_CONFIG_DIR is set to
// configDir, or removed when configDir is "" so Claude Code uses ~/.claude,
// and ProfileVar is set to name.
//
// Inherited ANTHROPIC_* variables and CLAUDE_CODE_OAUTH_TOKEN are removed.
// They choose the endpoint, credentials, and models, which the profile's
// settings define; Claude Code exports settings env to its children, so a
// profile started from inside another profile's session would otherwise run
// on the outer profile's gateway or login.
func Env(env []string, name, configDir string) []string {
	out := make([]string, 0, len(env)+2)
	for _, e := range env {
		key, _, _ := strings.Cut(e, "=")
		if key == "CLAUDE_CONFIG_DIR" || key == ProfileVar || key == "CLAUDE_CODE_OAUTH_TOKEN" || strings.HasPrefix(key, "ANTHROPIC_") {
			continue
		}
		out = append(out, e)
	}
	if configDir != "" {
		out = append(out, "CLAUDE_CONFIG_DIR="+configDir)
	}
	return append(out, ProfileVar+"="+name)
}

// Resolve looks up name in $PATH like exec.LookPath, skipping skipDir so that
// resolving "claude" never finds the ccs shim in ~/.ccs/bin.
func Resolve(name, skipDir string) (string, error) {
	skip, err := filepath.Abs(skipDir)
	if err != nil {
		return "", err
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		if abs, err := filepath.Abs(dir); err != nil || abs == skip {
			continue
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}
