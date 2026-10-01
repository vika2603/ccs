package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newUseCmd() *cobra.Command {
	var none bool
	cmd := &cobra.Command{
		Use:   "use <name> | use --none",
		Short: "Set the active profile that `claude` runs with (--none for ~/.claude)",
		Args: func(cmd *cobra.Command, args []string) error {
			if none {
				if len(args) > 0 {
					return fmt.Errorf("--none takes no profile name, got %q", args[0])
				}
				return nil
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			if none {
				return a.ClearActive()
			}
			if _, err := a.mgr.Path(args[0]); err != nil {
				return err
			}
			return a.SetActive(args[0])
		},
	}
	cmd.Flags().BoolVar(&none, "none", false, "clear the active profile")
	return cmd
}
