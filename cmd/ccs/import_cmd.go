package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/fields"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/state"
	"github.com/vika2603/ccs/internal/tui"
)

func newImportCmd() *cobra.Command {
	var move bool
	cmd := &cobra.Command{
		Use:   "import <src-dir> <name>",
		Short: "Adopt an existing .claude-style directory as a profile",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			src, name := args[0], args[1]
			if err := state.ValidName(name); err != nil {
				return err
			}
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			cfg, err := config.Load(p.ConfigFile())
			if err != nil {
				return err
			}
			reg := fields.NewRegistry(cfg.Fields)

			if _, err := os.Stat(src); err != nil {
				return err
			}
			dst := p.ProfilePath(name)
			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf("profile %q already exists", name)
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}

			prompter := importPrompter{
				out: cmd.OutOrStdout(),
				in:  bufferedStdin(cmd.InOrStdin()),
				err: cmd.ErrOrStderr(),
			}
			return fields.ImportEntries(src, dst, p.SharedDir(), reg, prompter, move)
		},
	}
	cmd.Flags().BoolVar(&move, "move", false, "move files instead of copying")
	return cmd
}

type importPrompter struct {
	out io.Writer
	in  io.Reader
	err io.Writer
}

func (p importPrompter) OnSharedConflict(name, existingPath, incomingPath string) (bool, error) {
	res, err := tui.PromptConflict(
		tui.Entry{Name: name, Path: existingPath},
		tui.Entry{Name: name, Path: incomingPath},
		p.out, p.in,
	)
	if err != nil {
		return false, err
	}
	return res == tui.ResolveOverwrite, nil
}

func (p importPrompter) OnUnknownEntry(name string) {
	fmt.Fprintf(p.err, "note: unknown entry %q treated as isolated\n", name)
}
