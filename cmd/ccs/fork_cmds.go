package main

import (
	"bufio"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/fields"
)

func bufferedStdin(r io.Reader) io.Reader {
	if _, ok := r.(*bufio.Reader); ok {
		return r
	}
	return bufio.NewReader(r)
}

// confirmed reads one line from r and reports whether it is a yes answer.
func confirmed(r io.Reader) bool {
	line, _ := bufio.NewReader(r).ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes"
}

// appForProfile loads the app and resolves name to itself or, when empty, to
// the active profile.
func appForProfile(name string) (app, string, error) {
	a, err := loadApp()
	if err != nil {
		return app{}, "", err
	}
	name, err = a.profileOrActive(name)
	return a, name, err
}

// argOrEmpty returns args[0], or "" when args is empty.
func argOrEmpty(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func newForkCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "fork <field> [<profile>]",
		Short:             "Break a shared symlink by copying shared/<field> into the profile",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeFieldThenProfile,
		RunE: func(cmd *cobra.Command, args []string) error {
			field := args[0]
			var name string
			if len(args) == 2 {
				name = args[1]
			}
			a, profile, err := appForProfile(name)
			if err != nil {
				return err
			}
			if err := a.ops().Fork(profile, field); err != nil {
				return err
			}
			cmd.Printf("forked %s for profile %s\n", field, profile)
			return nil
		},
	}
}

func newShareCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "share <field> [<profile>]",
		Short:             "Push a forked field back into shared/ and relink the profile",
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completeFieldThenProfile,
		RunE: func(cmd *cobra.Command, args []string) error {
			field := args[0]
			var name string
			if len(args) == 2 {
				name = args[1]
			}
			a, profile, err := appForProfile(name)
			if err != nil {
				return err
			}
			if err := a.ops().Share(profile, field, importPrompter{out: cmd.OutOrStdout(), in: bufferedStdin(cmd.InOrStdin())}.OnSharedConflict); err != nil {
				return err
			}
			cmd.Printf("shared %s for profile %s\n", field, profile)
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "status [<profile>]",
		Short:             "Report per-field link state and summarize shared/",
		Args:              cobra.RangeArgs(0, 1),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) == 1 {
				name = args[0]
			}
			a, profile, err := appForProfile(name)
			if err != nil {
				return err
			}
			st, err := a.ops().Status(profile)
			if err != nil {
				return err
			}
			if account := a.mgr.Account(profile); account != "" {
				cmd.Printf("profile %s (%s)\n", profile, account)
			} else {
				cmd.Printf("profile %s\n", profile)
			}
			for _, field := range slices.Sorted(maps.Keys(st)) {
				cmd.Printf("  %s\t%s\n", field, describeLinkState(st[field]))
			}

			sharedEntries, err := os.ReadDir(a.SharedDir())
			if err != nil {
				return err
			}
			cmd.Println("shared/:")
			for _, e := range sharedEntries {
				info, err := e.Info()
				if err != nil {
					continue
				}
				if e.IsDir() {
					children, _ := os.ReadDir(filepath.Join(a.SharedDir(), e.Name()))
					cmd.Printf("  %s/\t(%d entries)\n", e.Name(), len(children))
					continue
				}
				cmd.Printf("  %s\t(%d bytes)\n", e.Name(), info.Size())
			}
			return nil
		},
	}
}

func describeLinkState(s fields.LinkState) string {
	switch s {
	case fields.Linked:
		return "linked"
	case fields.Forked:
		return "forked"
	default:
		return "missing"
	}
}
