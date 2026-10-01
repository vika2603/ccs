package main

import (
	"os"
	"syscall"

	"github.com/vika2603/ccs/internal/profileenv"
)

// launch execs the command in rest (default: the configured launch command)
// with CLAUDE_CONFIG_DIR and the env vars of profile name. An empty name
// passes the parent environment through unchanged, so the PATH shim can
// still start claude when no profile is active.
func (a app) launch(name string, rest []string) error {
	rest = a.launchCommand(rest)
	bin, err := profileenv.ResolveSkipping(rest, []string{a.BinDir()})
	if err != nil {
		return err
	}
	env := os.Environ()
	if name != "" {
		path, err := a.mgr.Path(name)
		if err != nil {
			return err
		}
		penv, err := profileenv.Load(a.EnvFile(name))
		if err != nil {
			return err
		}
		env = profileenv.BuildEnv(env, path, penv.Env)
	}
	return syscall.Exec(bin, rest, env)
}

func (a app) launchCommand(rest []string) []string {
	if len(rest) > 0 {
		return rest
	}
	if len(a.cfg.Launch.Command) > 0 {
		return append([]string{}, a.cfg.Launch.Command...)
	}
	return []string{"claude"}
}

// splitProfileArgs splits `<profile> [--] [args...]` into the profile name
// and the remaining arguments.
func splitProfileArgs(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	rest := args[1:]
	if len(rest) > 0 && rest[0] == "--" {
		rest = rest[1:]
	}
	return args[0], rest
}
