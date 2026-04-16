//go:build !darwin

package main

import "github.com/spf13/cobra"

func newKeychainPruneCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "prune",
		Short:  "Keychain prune (macOS only)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.Println("keychain prune is only supported on macOS")
			return nil
		},
	}
}
