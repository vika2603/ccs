package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/runx"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "run <profile> -- <cmd> [args...]",
		Short:              "Run a command with CLAUDE_CONFIG_DIR set to a profile",
		DisableFlagParsing: false,
		Args:               cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			rest := args[1:]
			if len(rest) > 0 && rest[0] == "--" {
				rest = rest[1:]
			}
			if len(rest) == 0 {
				return fmt.Errorf("run: command missing after --")
			}
			m, _, err := manager()
			if err != nil {
				return err
			}
			path, err := m.Path(name)
			if err != nil {
				return err
			}
			bin, err := runx.Resolve(rest)
			if err != nil {
				return err
			}
			env := runx.BuildEnv(os.Environ(), path)
			return syscall.Exec(bin, rest, env)
		},
	}
	return cmd
}
