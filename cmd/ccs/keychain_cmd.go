package main

import (
	"github.com/spf13/cobra"
)

func newKeychainCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "keychain",
		Short: "Keychain utilities",
	}
	root.AddCommand(newKeychainPruneCmd())
	return root
}
