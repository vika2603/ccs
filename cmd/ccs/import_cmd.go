package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/config"
	"github.com/vika2603/ccs/internal/creds"
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
			reg := fields.NewRegistry(cfg)

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

			in := bufferedStdin(cmd.InOrStdin())
			prompter := importPrompter{
				out: cmd.OutOrStdout(),
				in:  in,
				err: cmd.ErrOrStderr(),
			}
			if err := fields.ImportEntries(src, dst, p.SharedDir(), reg, prompter, move); err != nil {
				return err
			}
			if err := maybeImportClaudeJSON(src, dst, name, in, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
			return maybeImportCreds(src, dst, name, move, creds.New(), in, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().BoolVar(&move, "move", false, "move files instead of copying")
	return cmd
}

// maybeImportClaudeJSON handles the legacy default Claude Code layout where
// .claude.json lives at $HOME/.claude.json (sibling of ~/.claude/), not inside
// the directory. With CLAUDE_CONFIG_DIR set, the file lives inside the config
// dir and is picked up by ImportEntries as a normal Isolated entry; in that
// case this helper is a no-op.
func maybeImportClaudeJSON(src, dst, name string, in io.Reader, out, errOut io.Writer) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(absSrc, ".claude.json")); err == nil {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	if absSrc != filepath.Join(home, ".claude") {
		return nil
	}
	siblingPath := filepath.Join(home, ".claude.json")
	data, err := os.ReadFile(siblingPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		fmt.Fprintf(errOut, "warning: could not read %s: %v\n", siblingPath, err)
		return nil
	}
	br, ok := in.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(in)
	}
	fmt.Fprintf(out, "found .claude.json sibling at %s; import to profile %q? (Y/n) ", siblingPath, name)
	line, _ := br.ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	if ans == "n" || ans == "no" {
		fmt.Fprintln(out, "skipped .claude.json")
		return nil
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return nil
	}
	dstPath := filepath.Join(absDst, ".claude.json")
	if err := os.WriteFile(dstPath, data, 0o600); err != nil {
		fmt.Fprintf(errOut, "warning: could not write .claude.json: %v\n", err)
		return nil
	}
	fmt.Fprintf(out, "imported %s -> %s (sibling preserved)\n", siblingPath, dstPath)
	return nil
}

func maybeImportCreds(src, dst, name string, move bool, store creds.Store, in io.Reader, out, errOut io.Writer) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return nil
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return nil
	}
	data, err := store.Read(absSrc)
	if errors.Is(err, creds.ErrNotFound) {
		return nil
	}
	if err != nil {
		fmt.Fprintf(errOut, "warning: could not read source credentials: %v\n", err)
		return nil
	}
	br, ok := in.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(in)
	}
	fmt.Fprintf(out, "found credentials for %s; import to profile %q? (y/N) ", src, name)
	line, _ := br.ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	if ans != "y" && ans != "yes" {
		fmt.Fprintln(out, "skipped credentials")
		return nil
	}
	if err := store.Write(absDst, data); err != nil {
		fmt.Fprintf(errOut, "warning: could not write credentials: %v\n", err)
		return nil
	}
	if move {
		if err := store.Delete(absSrc); err != nil {
			fmt.Fprintf(errOut, "warning: could not delete source credentials: %v\n", err)
		}
	}
	return nil
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
