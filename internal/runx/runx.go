package runx

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// BuildEnv returns env with CLAUDE_CONFIG_DIR forced to configDir and with
// profileEnv entries overlaid on top of any same-named entries already present.
// profileEnv may be nil.
func BuildEnv(env []string, configDir string, profileEnv map[string]string) []string {
	override := make(map[string]struct{}, len(profileEnv)+1)
	override["CLAUDE_CONFIG_DIR"] = struct{}{}
	for k := range profileEnv {
		override[k] = struct{}{}
	}
	out := make([]string, 0, len(env)+len(profileEnv)+1)
	for _, e := range env {
		name, _, ok := strings.Cut(e, "=")
		if !ok {
			out = append(out, e)
			continue
		}
		if _, ovr := override[name]; ovr {
			continue
		}
		out = append(out, e)
	}
	keys := make([]string, 0, len(profileEnv))
	for k := range profileEnv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+profileEnv[k])
	}
	out = append(out, "CLAUDE_CONFIG_DIR="+configDir)
	return out
}

func Resolve(argv []string) (string, error) {
	return ResolveSkipping(argv, nil)
}

// ResolveSkipping is like Resolve but walks $PATH manually and ignores any
// entries whose absolute form matches one of skipDirs. Used to keep `ccs run`
// from picking up its own shim at ~/.ccs/bin/claude when resolving "claude".
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

