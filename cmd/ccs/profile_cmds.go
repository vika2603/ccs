package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/profile"
	"github.com/vika2603/ccs/internal/state"
)

func manager() (profile.Manager, layout.Paths, error) {
	p, err := layout.FromEnv()
	if err != nil {
		return profile.Manager{}, layout.Paths{}, err
	}
	cfg, err := config.Load(p.ConfigFile())
	if err != nil {
		return profile.Manager{}, p, err
	}
	reg := fields.NewRegistry(cfg.Fields)
	return profile.NewManager(p, reg).WithCreds(creds.New()), p, nil
}

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the ccs directory structure",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, p, err := manager()
			if err != nil {
				return err
			}
			if err := m.Init(); err != nil {
				return err
			}
			if _, err := os.Stat(p.ConfigFile()); errors.Is(err, os.ErrNotExist) {
				if err := config.Save(p.ConfigFile(), config.Default()); err != nil {
					return err
				}
			} else if err == nil {
				cmd.Println("config.toml exists; leaving it.")
			} else {
				return err
			}
			cmd.Println("initialized", p.Root())
			return nil
		},
	}
}

func newNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, _, err := manager()
			if err != nil {
				return err
			}
			return m.New(args[0])
		},
	}
}

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			m, p, err := manager()
			if err != nil {
				return err
			}
			names, err := m.List()
			if err != nil {
				return err
			}
			active, _ := state.Read(p.ActiveFile())
			for _, n := range names {
				marker := "  "
				if n == active {
					marker = "* "
				}
				cmd.Printf("%s%s\n", marker, n)
			}
			return nil
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
			m, p, err := manager()
			if err != nil {
				return err
			}
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else {
				name, _ = state.Read(p.ActiveFile())
				if name == "" {
					return fmt.Errorf("no active profile")
				}
			}
			path, err := m.Path(name)
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
			m, p, err := manager()
			if err != nil {
				return err
			}
			active, _ := state.Read(p.ActiveFile())
			if active == name && !force {
				return fmt.Errorf("profile %q is active; use --force or `ccs use` first", name)
			}
			if !yes {
				cmd.Printf("remove profile %q? (y/N) ", name)
				var ans string
				fmt.Scanln(&ans)
				if ans != "y" && ans != "Y" {
					return fmt.Errorf("aborted")
				}
			}
			if err := m.Remove(name); err != nil {
				return err
			}
			if active == name {
				_ = state.Clear(p.ActiveFile())
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	cmd.Flags().BoolVar(&force, "force", false, "allow removing the active profile")
	return cmd
}

func newMvCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "mv <old> <new>",
		Short:             "Rename a profile",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, p, err := manager()
			if err != nil {
				return err
			}
			active, _ := state.Read(p.ActiveFile())
			if err := m.Rename(args[0], args[1]); err != nil {
				return err
			}
			if active == args[0] {
				return state.Write(p.ActiveFile(), args[1])
			}
			return nil
		},
	}
}
