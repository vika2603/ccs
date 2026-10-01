package main

import (
	"os"
	"syscall"

	"github.com/spf13/cobra"

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

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "run [profile] [-- <cmd> [args...]]",
		Short:             "Run a command with CLAUDE_CONFIG_DIR set (default profile: active, default cmd: claude)",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			name, rest := splitProfileArgs(args)
			if name == "" {
				// No explicit profile: use the active one if any, otherwise
				// pass through. This lets the PATH shim call `ccs run -- claude`
				// unconditionally.
				name, _ = a.Active()
			}
			return a.launch(name, rest)
		},
	}
}
