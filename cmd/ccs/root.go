package main

import (
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ccs [profile] [-- claude-args...]",
		Short: "Claude Code profile switcher (bare `ccs` or `ccs <profile>` launches claude)",
		Example: `  ccs                  launch claude with the active profile
  ccs work             launch claude with profile "work"
  ccs work -- -c       same, passing -c to claude
  ccs -- -c            pass -c to claude with the active profile,
                       or to the default ~/.claude when none is active`,
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
			// `ccs -- args...`: no profile named, so behave like `claude args...`
			// through the shim: an explicit CLAUDE_CONFIG_DIR wins, then the
			// active profile, then plain claude.
			if cmd.ArgsLenAtDash() == 0 {
				name := ""
				if os.Getenv("CLAUDE_CONFIG_DIR") == "" {
					name, _ = a.Active()
				}
				return a.launch(name, append(a.launchCommand(nil), args...))
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
	root.AddCommand(
		newInitCmd(), newNewCmd(), newLsCmd(), newUseCmd(), newRmCmd(), newMvCmd(),
		newStatusCmd(), newEnvCmd(), newFieldCmd(), newDoctorCmd(),
		newBackupCmd(), newRestoreCmd(),
		newInternalShimExecCmd(),
	)
	return root
}
