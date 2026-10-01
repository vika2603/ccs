package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	var blank, move bool
	var from string
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a profile: empty, cloned from a profile, or adopted from a directory",
		Example: `  ccs new work                    empty profile linked to the shared assets
  ccs new work --blank            empty profile without shared links
  ccs new work2 --from work       clone profile "work"
  ccs new home --from ~/.claude   adopt an existing Claude Code directory`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			name := args[0]
			switch {
			case from == "" && move:
				return errors.New("--move requires --from <dir>")
			case from == "":
				return a.mgr.New(name, blank)
			case blank:
				return errors.New("--blank cannot be combined with --from")
			case a.isProfileRef(from):
				if move {
					return errors.New("--move only applies when --from is a directory")
				}
				return a.mgr.Clone(from, name)
			default:
				if info, err := os.Stat(from); err != nil || !info.IsDir() {
					return fmt.Errorf("--from %q is neither a profile nor a directory", from)
				}
				return a.adopt(cmd, from, name, move)
			}
		},
	}
	cmd.Flags().BoolVarP(&blank, "blank", "b", false, "do not link the shared assets")
	cmd.Flags().StringVar(&from, "from", "", "profile to clone, or Claude Code directory to adopt")
	cmd.Flags().BoolVar(&move, "move", false, "with --from <dir>, move files instead of copying")
	// Offer profile names and still let the shell complete directories.
	_ = cmd.RegisterFlagCompletionFunc("from", func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		names, _ := completeProfileNames(c, args, toComplete)
		return names, cobra.ShellCompDirectiveDefault
	})
	return cmd
}

// isProfileRef reports whether from names an existing profile rather than a
// directory. Anything containing a path separator is a directory, so
// `--from ./work` adopts a directory even when a profile "work" exists.
func (a app) isProfileRef(from string) bool {
	if strings.ContainsRune(from, filepath.Separator) || from == "." || from == ".." {
		return false
	}
	ok, err := a.mgr.Exists(from)
	return err == nil && ok
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
				if account := a.mgr.Account(n); account != "" {
					fmt.Fprintf(tw, "%s %s\t%s\n", marker, n, account)
				} else {
					fmt.Fprintf(tw, "%s %s\n", marker, n)
				}
			}
			return tw.Flush()
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
				return fmt.Errorf("profile %q is active; use --force or switch with `ccs use <name>` first", name)
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
