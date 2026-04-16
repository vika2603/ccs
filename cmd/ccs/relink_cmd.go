package main

import (
	"github.com/spf13/cobra"
)

func newRelinkCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:               "relink [<field>] [<profile>]",
		Short:             "Create the symlink from profile/<field> to shared/<field> when it is missing",
		Args:              cobra.RangeArgs(0, 2),
		ValidArgsFunction: completeFieldThenProfile,
		RunE: func(cmd *cobra.Command, args []string) error {
			var field, name string
			switch len(args) {
			case 0:
			case 1:
				if all {
					name = args[0]
				} else {
					field = args[0]
				}
			case 2:
				field = args[0]
				name = args[1]
			}
			ops, profile, err := opsForActive(name)
			if err != nil {
				return err
			}
			if all {
				relinked, err := ops.RelinkAll(profile)
				if err != nil {
					return err
				}
				if len(relinked) == 0 {
					cmd.Printf("nothing to relink for profile %s\n", profile)
					return nil
				}
				for _, f := range relinked {
					cmd.Printf("relinked %s for profile %s\n", f, profile)
				}
				return nil
			}
			if field == "" {
				return cmd.Usage()
			}
			if err := ops.Relink(profile, field); err != nil {
				return err
			}
			cmd.Printf("relinked %s for profile %s\n", field, profile)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Relink every missing shared field for the profile")
	return cmd
}
