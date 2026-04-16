package main

import (
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "ccs",
		Short:         "Claude Code profile switcher",
		SilenceUsage:  true,
		SilenceErrors: true,
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
