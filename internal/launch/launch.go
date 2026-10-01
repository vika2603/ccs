// Package launch builds the environment and resolves the binary used to exec
// claude.
package launch

import (
	"fmt"
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

// ResolveSkipping resolves argv[0] like exec.LookPath but ignores any $PATH
// entries whose absolute form matches one of skipDirs. Used to keep ccs from
// picking up its own shim at ~/.ccs/bin/claude when resolving "claude".
//
// If argv[0] contains a slash, it's returned as-is (matching exec.LookPath's
// behavior for explicit paths). If skipDirs is empty, falls back to
// exec.LookPath so callers don't pay for manual PATH walking.
func ResolveSkipping(argv []string, skipDirs []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("run: no command given")
	}
	name := argv[0]
	if strings.ContainsRune(name, '/') {
		return name, nil
	}
	if len(skipDirs) == 0 {
		return exec.LookPath(name)
	}
	skip := make(map[string]struct{}, len(skipDirs))
	for _, d := range skipDirs {
		abs, err := filepath.Abs(d)
		if err != nil {
			continue
		}
		skip[abs] = struct{}{}
	}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			dir = "."
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if _, ok := skip[abs]; ok {
			continue
		}
		candidate := filepath.Join(dir, name)
		if isExecutable(candidate) {
			return candidate, nil
		}
	}
	return "", &exec.Error{Name: name, Err: exec.ErrNotFound}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
