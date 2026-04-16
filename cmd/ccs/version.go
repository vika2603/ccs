package main

import "github.com/spf13/cobra"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print ccs version",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("ccs", Version)
			return nil
		},
	}
}
