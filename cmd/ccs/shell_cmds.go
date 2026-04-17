package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/profileenv"
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
		Use:               "use [name]",
		Short:             "Switch active profile",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeProfileNamesAtArg0,
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
				fmt.Fprint(cmd.OutOrStdout(), profileenv.RenderClearAll())
				return nil
			}
			name := args[0]
			path, err := m.Path(name)
			if err != nil {
				return err
			}
			envFile := p.EnvFile(name)
			penv, err := profileenv.Load(envFile)
			if err != nil {
				return err
			}
			if err := state.Write(p.ActiveFile(), name); err != nil {
				return err
			}
			out := profileenv.Render(profileenv.Action{
				Set:       penv.Env,
				ConfigDir: path,
				Sig:       profileenv.Signature(name, envFile),
			})
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

// newInternalShellHookCmd implements the `ccs __shell_hook` command called by
// the prompt hook (see internal/shell.zshSnippet). It prints shell code that,
// when eval'd, brings the shell's env in line with the active profile. If the
// shell is already in sync (CCS_ENV_SIG matches), it prints nothing.
func newInternalShellHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "__shell_hook",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, p, err := manager()
			if err != nil {
				// Hook runs on every prompt; don't surface errors to the user.
				return nil
			}
			active, _ := state.Read(p.ActiveFile())
			// If state points at a missing profile directory, treat as no active.
			var profilePath string
			if active != "" {
				dir, perr := m.Path(active)
				if perr != nil {
					active = ""
				} else {
					profilePath = dir
				}
			}

			haveCCD := os.Getenv("CLAUDE_CONFIG_DIR")
			haveManagedCCD := os.Getenv("CCS_MANAGED_CCD")
			// User owns CLAUDE_CONFIG_DIR. Treat as "opted out" and skip env sync
			// too - syncing envs while ignoring the user's CCD choice would be a
			// half-sync that surprises more than it helps.
			if haveCCD != "" && haveManagedCCD == "" {
				return nil
			}

			envFile := p.EnvFile(active)
			wantSig := profileenv.Signature(active, envFile)
			if os.Getenv("CCS_ENV_SIG") == wantSig {
				return nil
			}

			if active == "" {
				fmt.Fprint(cmd.OutOrStdout(), profileenv.RenderClearManaged())
				return nil
			}

			penv, err := profileenv.Load(envFile)
			if err != nil {
				return nil
			}
			out := profileenv.Render(profileenv.Action{
				Set:       penv.Env,
				ConfigDir: profilePath,
				Sig:       wantSig,
			})
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}
