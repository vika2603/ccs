package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/runx"
	"github.com/vika2603/ccs/internal/state"
)

func runClaudeForProfile(name string, rest []string) error {
	m, p, err := manager()
	if err != nil {
		return err
	}
	path, err := m.Path(name)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		cfg, err := config.Load(p.ConfigFile())
		if err != nil {
			return err
		}
		if len(cfg.Launch.Command) > 0 {
			rest = append([]string{}, cfg.Launch.Command...)
		} else {
			rest = []string{"claude"}
		}
	}
	bin, err := runx.Resolve(rest)
	if err != nil {
		return err
	}
	env := runx.BuildEnv(os.Environ(), path)
	return syscall.Exec(bin, rest, env)
}

func activeProfileName() (string, error) {
	_, p, err := manager()
	if err != nil {
		return "", err
	}
	name, _ := state.Read(p.ActiveFile())
	if name == "" {
		return "", fmt.Errorf("no active profile; run `ccs use <profile>` or pass a profile name")
	}
	return name, nil
}

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "run [profile] [-- <cmd> [args...]]",
		Short:              "Run a command with CLAUDE_CONFIG_DIR set (default profile: active, default cmd: claude)",
		DisableFlagParsing: false,
		Args:               cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			var rest []string
			if len(args) == 0 {
				n, err := activeProfileName()
				if err != nil {
					return err
				}
				name = n
			} else {
				name = args[0]
				rest = args[1:]
				if len(rest) > 0 && rest[0] == "--" {
					rest = rest[1:]
				}
			}
			return runClaudeForProfile(name, rest)
		},
	}
	return cmd
}
