package main

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create ~/.ccs and install the claude shim (safe to re-run)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			if err := a.mgr.Init(); err != nil {
				return err
			}
			if _, err := os.Stat(a.ConfigFile()); errors.Is(err, os.ErrNotExist) {
				if err := config.Save(a.ConfigFile(), config.Default()); err != nil {
					return err
				}
			} else if err == nil {
				cmd.Println("config.toml exists; leaving it.")
			} else {
				return err
			}
			shim, err := a.installShim()
			if err != nil {
				return err
			}
			cmd.Println("initialized", a.Root())
			cmd.Println("installed", shim)
			if !onPath(a.BinDir()) {
				cmd.Printf("\nAdd %s to PATH so `claude` runs the active profile. For GUI apps\n", a.BinDir())
				cmd.Printf("(VS Code, JetBrains) to see it too, put this in ~/.zprofile:\n\n")
				cmd.Printf("  export PATH=\"%s:$PATH\"\n", a.BinDir())
			}
			return nil
		},
	}
}

func newNewCmd() *cobra.Command {
	var blank bool
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			return a.mgr.New(args[0], blank)
		},
	}
	cmd.Flags().BoolVarP(&blank, "blank", "b", false, "create a blank profile without linking shared assets")
	return cmd
}

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List profiles with their account (* marks the active one)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			names, err := a.mgr.List()
			if err != nil {
				return err
			}
			active, _ := a.Active()
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, n := range names {
				marker := " "
				if n == active {
					marker = "*"
				}
				fmt.Fprintf(tw, "%s %s\t%s\n", marker, n, a.mgr.Account(n))
			}
			return tw.Flush()
		},
	}
}

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "path [name]",
		Short:             "Print a profile's absolute path (default: active)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else {
				name, _ = a.Active()
				if name == "" {
					return fmt.Errorf("no active profile")
				}
			}
			path, err := a.mgr.Path(name)
			if err != nil {
				return err
			}
			cmd.Println(path)
			return nil
		},
	}
}

func newRmCmd() *cobra.Command {
	var yes bool
	var force bool
	cmd := &cobra.Command{
		Use:               "rm <name>",
		Short:             "Remove a profile",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			a, err := loadApp()
			if err != nil {
				return err
			}
			active, _ := a.Active()
			if active == name && !force {
				return fmt.Errorf("profile %q is active; use --force or `ccs use` first", name)
			}
			if !yes {
				cmd.Printf("remove profile %q? (y/N) ", name)
				if !confirmed(cmd.InOrStdin()) {
					return errors.New("aborted")
				}
			}
			if err := a.mgr.Remove(name); err != nil {
				return err
			}
			if active == name {
				_ = a.ClearActive()
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	cmd.Flags().BoolVar(&force, "force", false, "allow removing the active profile")
	return cmd
}

func newCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "clone <source> <new>",
		Short:             "Clone an existing profile",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			return a.mgr.Clone(args[0], args[1])
		},
	}
}

func newMvCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "mv <old> <new>",
		Short:             "Rename a profile",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			active, _ := a.Active()
			if err := a.mgr.Rename(args[0], args[1]); err != nil {
				return err
			}
			if active == args[0] {
				return a.SetActive(args[1])
			}
			return nil
		},
	}
}
