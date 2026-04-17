package runx

import (
	"fmt"
	"os/exec"
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
	if len(argv) == 0 {
		return "", fmt.Errorf("run: no command given")
	}
	return exec.LookPath(argv[0])
}
