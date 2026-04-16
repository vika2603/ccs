package main

import (
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:               "ccs [profile]",
		Short:             "Claude Code profile switcher (bare `ccs` or `ccs <profile>` launches claude)",
		SilenceUsage:      true,
		SilenceErrors:     true,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				name, err := activeProfileName()
				if err != nil {
					cmd.Println(err)
					cmd.Println()
					return cmd.Help()
				}
				return runClaudeForProfile(name, nil)
			}
			name := args[0]
			rest := args[1:]
			if len(rest) > 0 && rest[0] == "--" {
				rest = rest[1:]
			}
			return runClaudeForProfile(name, rest)
		},
	}
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd(), newNewCmd(), newLsCmd(), newPathCmd(), newRmCmd(), newMvCmd())
	root.AddCommand(newShellInitCmd(), newUseCmd(), newInternalShellUseCmd())
	root.AddCommand(newRunCmd())
	root.AddCommand(newForkCmd(), newShareCmd(), newStatusCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newRestoreCmd())
	root.AddCommand(newDoctorCmd(), newKeychainCmd())
	return root
}
