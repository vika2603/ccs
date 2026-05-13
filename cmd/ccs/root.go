package main

import (
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:               "ccs [profile] [-- claude-args...]",
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
			claudeArgs := args[1:]
			if len(claudeArgs) > 0 && claudeArgs[0] == "--" {
				claudeArgs = claudeArgs[1:]
			}
			if len(claudeArgs) == 0 {
				return runClaudeForProfile(name, nil)
			}
			_, p, err := manager()
			if err != nil {
				return err
			}
			return runClaudeForProfile(name, append(defaultCommand(p, nil), claudeArgs...))
		},
	}
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.Flags().SetInterspersed(false)
	root.AddCommand(newVersionCmd())
	root.AddCommand(newInitCmd(), newNewCmd(), newLsCmd(), newPathCmd(), newRmCmd(), newMvCmd(), newCloneCmd())
	root.AddCommand(newShellInitCmd(), newUseCmd(), newUnuseCmd(), newInternalShellUseCmd(), newInternalShellUnuseCmd(), newInternalShellHookCmd())
	root.AddCommand(newRunCmd(), newInstallShimCmd(), newInternalShimExecCmd())
	root.AddCommand(newForkCmd(), newShareCmd(), newRelinkCmd(), newClassifyCmd(), newStatusCmd())
	root.AddCommand(newAdoptCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newBackupCmd())
	root.AddCommand(newRestoreCmd())
	root.AddCommand(newDoctorCmd(), newKeychainCmd())
	root.AddCommand(newEnvCmd())
	return root
}
