package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/state"
)

func bufferedStdin(r io.Reader) io.Reader {
	if _, ok := r.(*bufio.Reader); ok {
		return r
	}
	return bufio.NewReader(r)
}

func opsForActive(name string) (fields.Ops, string, error) {
	p, err := layout.FromEnv()
	if err != nil {
		return fields.Ops{}, "", err
	}
	cfg, err := config.Load(p.ConfigFile())
	if err != nil {
		return fields.Ops{}, "", err
	}
	if name == "" {
		active, err := state.Read(p.ActiveFile())
		if err != nil {
			return fields.Ops{}, "", err
		}
		if active == "" {
			return fields.Ops{}, "", fmt.Errorf("no active profile; pass a profile name explicitly")
		}
		name = active
	}
	reg := fields.NewRegistry(cfg.Fields)
	return fields.NewOps(p, reg), name, nil
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
			ops, profile, err := opsForActive(name)
			if err != nil {
				return err
			}
			if err := ops.Fork(profile, field); err != nil {
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
			ops, profile, err := opsForActive(name)
			if err != nil {
				return err
			}
			if err := ops.Share(profile, field, cmd.OutOrStdout(), bufferedStdin(cmd.InOrStdin())); err != nil {
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
			ops, profile, err := opsForActive(name)
			if err != nil {
				return err
			}
			st, err := ops.Status(profile)
			if err != nil {
				return err
			}
			cmd.Printf("profile %s\n", profile)
			for field, ls := range st {
				cmd.Printf("  %s\t%s\n", field, describeLinkState(ls))
			}

			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			sharedEntries, err := os.ReadDir(p.SharedDir())
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
					children, _ := os.ReadDir(filepath.Join(p.SharedDir(), e.Name()))
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
