package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/profileenv"
)

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage per-profile environment variables",
	}
	cmd.AddCommand(
		newEnvLsCmd(),
		newEnvSetCmd(),
		newEnvUnsetCmd(),
		newEnvEditCmd(),
	)
	return cmd
}

func maskValue(v string) string {
	const clip = 4
	n := len(v)
	if n == 0 {
		return ""
	}
	if n <= 2*clip {
		return strings.Repeat("*", n)
	}
	return v[:clip] + strings.Repeat("*", n-2*clip) + v[n-clip:]
}

func newEnvLsCmd() *cobra.Command {
	var showValues bool
	cmd := &cobra.Command{
		Use:               "ls [profile]",
		Short:             "List env var names (values masked by default)",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			name, err := a.profileOrActive(argOrEmpty(args))
			if err != nil {
				return err
			}
			f, err := profileenv.Load(a.EnvFile(name))
			if err != nil {
				return err
			}
			for _, k := range f.Keys() {
				if showValues {
					cmd.Printf("%s=%s\n", k, f.Env[k])
				} else {
					cmd.Printf("%s=%s\n", k, maskValue(f.Env[k]))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showValues, "show-values", false, "print full values (sensitive!)")
	return cmd
}

func newEnvSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "set <profile> <KEY=VALUE>...",
		Short:             "Set one or more env vars for a profile",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			name := args[0]
			if _, err := a.mgr.Path(name); err != nil {
				return err
			}
			f, err := profileenv.Load(a.EnvFile(name))
			if err != nil {
				return err
			}
			if f.Env == nil {
				f.Env = map[string]string{}
			}
			for _, arg := range args[1:] {
				k, v, err := profileenv.ParseAssignment(arg)
				if err != nil {
					return err
				}
				f.Env[k] = v
			}
			return profileenv.Save(a.EnvFile(name), f)
		},
	}
	return cmd
}

func newEnvUnsetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "unset <profile> <KEY>...",
		Short:             "Remove one or more env vars from a profile",
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			name := args[0]
			if _, err := a.mgr.Path(name); err != nil {
				return err
			}
			f, err := profileenv.Load(a.EnvFile(name))
			if err != nil {
				return err
			}
			for _, k := range args[1:] {
				if err := profileenv.ValidName(k); err != nil {
					return err
				}
				delete(f.Env, k)
			}
			return profileenv.Save(a.EnvFile(name), f)
		},
	}
	return cmd
}

func newEnvEditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "edit <profile>",
		Short:             "Open the env file in $EDITOR",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeProfileNamesAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := loadApp()
			if err != nil {
				return err
			}
			name := args[0]
			if _, err := a.mgr.Path(name); err != nil {
				return err
			}
			path := a.EnvFile(name)
			// Snapshot (or mark as "did not exist") so we can roll back on a
			// post-edit validation failure.
			var backup []byte
			hadBackup := true
			if b, rerr := os.ReadFile(path); rerr == nil {
				backup = b
			} else if errors.Is(rerr, os.ErrNotExist) {
				hadBackup = false
				if err := profileenv.Save(path, profileenv.File{Env: map[string]string{}}); err != nil {
					return err
				}
			} else {
				return rerr
			}
			editor := os.Getenv("VISUAL")
			if editor == "" {
				editor = os.Getenv("EDITOR")
			}
			if editor == "" {
				editor = "vi"
			}
			// git-style EDITOR invocation: editor itself may contain args
			// (e.g. "code --wait"), so let the shell split it, but pass the
			// path as a positional so it is not re-split or re-expanded.
			ec := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
			ec.Stdin = os.Stdin
			ec.Stdout = os.Stdout
			ec.Stderr = os.Stderr
			if err := ec.Run(); err != nil {
				return err
			}
			if _, err := profileenv.Load(path); err != nil {
				// Restore previous contents so the profile stays usable.
				if hadBackup {
					_ = os.WriteFile(path, backup, 0o600)
				} else {
					_ = os.Remove(path)
				}
				return fmt.Errorf("edited file is invalid (restored previous contents): %w", err)
			}
			return nil
		},
	}
	return cmd
}
