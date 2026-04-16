package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/shell"
	"github.com/vika2603/ccs/internal/state"
)

func newShellInitCmd() *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "shell-init",
		Short: "Print shell integration code",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s := shell.Zsh
			if kind == "bash" {
				s = shell.Bash
			} else if kind == "" {
				s = shell.Detect(os.Getenv("SHELL"))
			}
			fmt.Fprint(cmd.OutOrStdout(), shell.Render(s))
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "shell", "", "override detected shell (zsh|bash)")
	return cmd
}

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use [name]",
		Short: "Switch active profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := manager()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				return state.Clear(p.ActiveFile())
			}
			m, _, _ := manager()
			if _, err := m.Path(args[0]); err != nil {
				return err
			}
			return state.Write(p.ActiveFile(), args[0])
		},
	}
}

func newInternalShellUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "__shell_use [name]",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, p, err := manager()
			if err != nil {
				return err
			}
			if len(args) == 0 {
				if err := state.Clear(p.ActiveFile()); err != nil {
					return err
				}
				cmd.Println("unset CLAUDE_CONFIG_DIR CCS_MANAGED_CCD")
				return nil
			}
			path, err := m.Path(args[0])
			if err != nil {
				return err
			}
			if err := state.Write(p.ActiveFile(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "export CLAUDE_CONFIG_DIR=%s; export CCS_MANAGED_CCD=1\n", shellQuote(path))
			return nil
		},
	}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	out := "'"
	for _, c := range s {
		if c == '\'' {
			out += `'\''`
		} else {
			out += string(c)
		}
	}
	return out + "'"
}
