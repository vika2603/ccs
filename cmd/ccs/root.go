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
		Version:           Version,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			name, rest := splitProfileArgs(args)
			if name == "" {
				if name, err = a.profileOrActive(""); err != nil {
					cmd.Println(err)
					cmd.Println()
					return cmd.Help()
				}
			}
			if len(rest) > 0 {
				rest = append(a.launchCommand(nil), rest...)
			}
			return a.launch(name, rest)
		},
	}
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.Flags().SetInterspersed(false)
	root.SetVersionTemplate("ccs {{.Version}}\n")
	root.AddCommand(newInitCmd(), newNewCmd(), newLsCmd(), newPathCmd(), newRmCmd(), newMvCmd(), newCloneCmd())
	root.AddCommand(newShellInitCmd(), newUseCmd(), newUnuseCmd())
	root.AddCommand(newRunCmd(), newInternalShimExecCmd())
	root.AddCommand(newForkCmd(), newShareCmd(), newClassifyCmd(), newStatusCmd())
	root.AddCommand(newAdoptCmd())
	root.AddCommand(newImportCmd())
	root.AddCommand(newExportCmd())
	root.AddCommand(newBackupCmd())
	root.AddCommand(newRestoreCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newEnvCmd())
	return root
}
