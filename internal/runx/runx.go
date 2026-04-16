package runx

import (
	"fmt"
	"os/exec"
	"strings"
)

func BuildEnv(env []string, configDir string) []string {
	out := make([]string, 0, len(env)+1)
	seen := false
	for _, e := range env {
		if strings.HasPrefix(e, "CLAUDE_CONFIG_DIR=") {
			out = append(out, "CLAUDE_CONFIG_DIR="+configDir)
			seen = true
			continue
		}
		out = append(out, e)
	}
	if !seen {
		out = append(out, "CLAUDE_CONFIG_DIR="+configDir)
	}
	return out
}

func Resolve(argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("run: no command given")
	}
	return exec.LookPath(argv[0])
}
