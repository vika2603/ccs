package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/vika2603/ccs/internal/creds"
	"github.com/vika2603/ccs/internal/fsutil"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/profile"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Install the claude shim so `claude` runs the active profile (safe to re-run)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			shim, err := installShim(p)
			if err != nil {
				return err
			}
			cmd.Println("installed", shim)
			if !slices.Contains(filepath.SplitList(os.Getenv("PATH")), p.BinDir()) {
				cmd.Printf("\nPut %s first on PATH so `claude` runs the active profile. For GUI\n", p.BinDir())
				cmd.Printf("apps (VS Code, JetBrains) to see it too, add this to ~/.zprofile:\n\n")
				cmd.Printf("  export PATH=\"%s:$PATH\"\n", p.BinDir())
			}
			return nil
		},
	}
}

func newNewCmd() *cobra.Command {
	var login bool
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a profile and open it in $EDITOR",
		Long: `Create a profile file in ~/.ccs/profiles and open it in $EDITOR when run
from a terminal.

Without --login the profile runs Claude Code on ~/.claude and only adds the
settings you put in it, for example an API gateway's env. With --login it
gets its own config directory for a separate OAuth login; run it once
(ccs <name>) and log in with /login.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			name := args[0]
			if err := profile.Create(p, name, login); err != nil {
				return err
			}
			cmd.Println("created", p.ProfileFile(name))
			if isTerminal(cmd.InOrStdin()) {
				return editProfile(p, name)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&login, "login", false, "give the profile its own config directory for a separate OAuth login")
	return cmd
}

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "edit <name>",
		Short:             "Open a profile in $EDITOR",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProfileAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			if args[0] == layout.DefaultProfile {
				return errors.New("the default profile is ~/.claude itself; edit ~/.claude/settings.json instead")
			}
			if err := layout.ValidName(args[0]); err != nil {
				return err
			}
			// Only existence is checked: an unparsable file is what edit fixes.
			if _, err := os.Stat(p.ProfileFile(args[0])); err != nil {
				return fmt.Errorf("profile %q does not exist", args[0])
			}
			return editProfile(p, args[0])
		},
	}
}

// editProfile opens the profile file in $VISUAL or $EDITOR and restores the
// previous content if the result does not parse.
func editProfile(p layout.Paths, name string) error {
	path := p.ProfileFile(name)
	before, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	// The editor may carry arguments ("code --wait"), so the shell splits it;
	// the path is passed as a positional so it is not split or expanded.
	ec := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
	ec.Stdin, ec.Stdout, ec.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := ec.Run(); err != nil {
		return err
	}
	after, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if _, err := profile.Parse(after); err != nil {
		if werr := fsutil.WriteFileAtomic(path, before, 0o600); werr != nil {
			return errors.Join(err, werr)
		}
		return fmt.Errorf("%s is invalid, previous content restored: %w", path, err)
	}
	return nil
}

func newLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List profiles and their accounts (* marks the active one)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			names, err := profile.List(p)
			if err != nil {
				return err
			}
			active := activeProfile(p)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, name := range append([]string{layout.DefaultProfile}, names...) {
				marker := " "
				if name == active {
					marker = "*"
				}
				pr, err := profile.Load(p, name)
				if err != nil {
					fmt.Fprintf(tw, "%s %s\tinvalid: %v\n", marker, name, err)
					continue
				}
				kind := "~/.claude"
				if pr.Login {
					kind = "login"
				}
				fmt.Fprintf(tw, "%s %s\t%s\t%s\n", marker, name, kind, profile.Account(p, pr))
			}
			return tw.Flush()
		},
	}
}

func newUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "use <name>",
		Short:             "Set the profile that `claude` runs with (default: ~/.claude)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProfileAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			if args[0] == layout.DefaultProfile {
				return p.ClearActive()
			}
			if _, err := profile.Load(p, args[0]); err != nil {
				return err
			}
			return p.SetActive(args[0])
		},
	}
}

func newRmCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:               "rm <name>",
		Short:             "Remove a profile, and for a login profile its directory and stored login",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProfileAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			name := args[0]
			if !yes {
				cmd.Printf("remove profile %q? (y/N) ", name)
				if !confirmed(cmd.InOrStdin()) {
					return errors.New("aborted")
				}
			}
			if err := profile.Remove(p, name, creds.Delete); err != nil {
				return err
			}
			if active, _ := p.Active(); active == name {
				return p.ClearActive()
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	return cmd
}

// installShim writes ~/.ccs/bin/claude, a sh script that routes to
// `ccs __shim_exec claude`. It embeds the absolute path of the running ccs
// binary, so it keeps working if ccs drops out of PATH.
func installShim(p layout.Paths) (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate ccs binary: %w", err)
	}
	self, err = filepath.Abs(self)
	if err != nil {
		return "", err
	}
	shim := p.ShimPath()
	// Replace a symlink at the shim path itself; writing through it would
	// overwrite whatever it points to, possibly the real claude binary.
	if info, err := os.Lstat(shim); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(shim); err != nil {
			return "", err
		}
	}
	return shim, fsutil.WriteFileAtomic(shim, []byte(shimScript(self)), 0o755)
}

func shimScript(ccsPath string) string {
	return "#!/bin/sh\n" +
		"# Generated by `ccs init`. Runs claude with the active ccs profile.\n" +
		"exec " + shellQuote(ccsPath) + " __shim_exec claude \"$@\"\n"
}

// shellQuote wraps s in single quotes so sh reads it verbatim.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// confirmed reads one line from r and reports whether it is a yes answer.
func confirmed(r io.Reader) bool {
	line, _ := bufio.NewReader(r).ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes"
}

func isTerminal(r io.Reader) bool {
	f, ok := r.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}
