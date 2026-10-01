package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newShellInitCmd keeps `eval "$(ccs shell-init)"` lines in existing rc files
// working. Profile switching is handled entirely by the ~/.ccs/bin/claude
// shim, so the only thing left to install in the shell is tab completion.
func newShellInitCmd() *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:    "shell-init",
		Short:  "Print shell completion code (same as `ccs completion <shell>`)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if kind == "" {
				kind = "zsh"
				if strings.Contains(filepath.Base(os.Getenv("SHELL")), "bash") {
					kind = "bash"
				}
			}
			root := cmd.Root()
			switch kind {
			case "zsh":
				// The script calls compdef, which only exists after compinit;
				// skip registration instead of erroring on every shell start.
				var buf bytes.Buffer
				if err := root.GenZshCompletion(&buf); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "if (( $+functions[compdef] )); then\n%s\nfi\n", buf.String())
				return nil
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			default:
				return fmt.Errorf("unsupported shell %q; use zsh or bash", kind)
			}
		},
	}
	cmd.Flags().StringVar(&kind, "shell", "", "override detected shell (zsh|bash)")
	return cmd
}

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "use <name>",
		Short:             "Switch the active profile used by `claude`",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			if _, err := a.mgr.Path(args[0]); err != nil {
				return err
			}
			return a.SetActive(args[0])
		},
	}
}

func newUnuseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unuse",
		Short: "Deactivate the current profile",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			return a.ClearActive()
		},
	}
}
