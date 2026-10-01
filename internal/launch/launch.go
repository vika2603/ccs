// Package launch builds the environment and resolves the binary used to exec
// claude.
package launch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Env returns env with CLAUDE_CONFIG_DIR set to configDir, or removed when
// configDir is "" so Claude Code falls back to ~/.claude.
func Env(env []string, configDir string) []string {
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if name, _, _ := strings.Cut(e, "="); name == "CLAUDE_CONFIG_DIR" {
			continue
		}
		out = append(out, e)
	}
	if configDir != "" {
		out = append(out, "CLAUDE_CONFIG_DIR="+configDir)
	}
	return out
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
