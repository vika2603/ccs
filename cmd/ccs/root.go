package main

import (
	"os"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/vika2603/ccs/internal/launch"
	"github.com/vika2603/ccs/internal/layout"
	"github.com/vika2603/ccs/internal/profile"
)

var Version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "ccs [profile] [-- claude-args...]",
		Short: "Run Claude Code with different accounts on top of one ~/.claude",
		Example: `  ccs                  claude with the active profile
  ccs work             claude with profile "work"
  ccs work -- -c       same, passing -c to claude
  ccs -- -c            pass -c to claude with the active profile`,
		SilenceUsage:      true,
		SilenceErrors:     true,
		Version:           Version,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeProfileAtArg0,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			// A leading `--` names no profile: behave like `claude args...`
			// through the shim.
			if cmd.ArgsLenAtDash() == 0 {
				return runShim(p, args)
			}
			name, rest := activeProfile(p), []string(nil)
			if len(args) > 0 {
				name, rest = args[0], args[1:]
			}
			// Flag parsing stops at the profile name, so a separating `--`
			// after it arrives as a plain argument.
			if len(rest) > 0 && rest[0] == "--" {
				rest = rest[1:]
			}
			return run(p, name, rest)
		},
	}
	root.SetOut(os.Stdout)
	root.SetErr(os.Stderr)
	root.Flags().SetInterspersed(false)
	root.SetVersionTemplate("ccs {{.Version}}\n")
	root.AddCommand(
		newInitCmd(), newNewCmd(), newEditCmd(), newLsCmd(), newUseCmd(), newRmCmd(),
		newShimExecCmd(),
	)
	return root
}

// activeProfile returns the active profile, or the default profile when none
// is set.
func activeProfile(p layout.Paths) string {
	if name, _ := p.Active(); name != "" {
		return name
	}
	return layout.DefaultProfile
}

// run execs claude with profile name: its config directory (none for
// profiles that run on ~/.claude), its settings, and args.
func run(p layout.Paths, name string, args []string) error {
	pr, err := profile.Load(p, name)
	if err != nil {
		return err
	}
	if err := profile.Sync(p, pr); err != nil {
		return err
	}
	settings, err := profile.WriteSettings(p, pr)
	if err != nil {
		return err
	}
	argv := []string{"claude"}
	if settings != "" {
		// --settings must precede a claude subcommand; after it, the
		// subcommand rejects the flag.
		argv = append(argv, "--settings", settings)
	}
	return execClaude(p, append(argv, args...), launch.Env(os.Environ(), pr.ConfigDir(p)))
}

// runShim runs claude the way `claude args...` does through the PATH shim:
// a CLAUDE_CONFIG_DIR already set by the caller is used as is, otherwise the
// active profile applies.
func runShim(p layout.Paths, args []string) error {
	if os.Getenv("CLAUDE_CONFIG_DIR") != "" {
		return execClaude(p, append([]string{"claude"}, args...), os.Environ())
	}
	return run(p, activeProfile(p), args)
}

func execClaude(p layout.Paths, argv, env []string) error {
	bin, err := launch.Resolve(argv[0], p.BinDir())
	if err != nil {
		return err
	}
	return syscall.Exec(bin, argv, env)
}

// newShimExecCmd implements `ccs __shim_exec claude [args...]`, called by the
// ~/.ccs/bin/claude shim. A CLAUDE_CONFIG_DIR already in the environment is
// honored, so a wrapper that resolves `claude` back to the shim keeps the
// profile it was started with. Flag parsing is off because every argument
// belongs to claude.
func newShimExecCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "__shim_exec claude [args...]",
		Hidden:             true,
		DisableFlagParsing: true,
		Args:               cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := layout.FromEnv()
			if err != nil {
				return err
			}
			return runShim(p, args[1:])
		},
	}
}

func completeProfiles(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	p, err := layout.FromEnv()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names, err := profile.List(p)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	return append([]string{layout.DefaultProfile}, names...), cobra.ShellCompDirectiveNoFileComp
}

func completeProfileAtArg0(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return completeProfiles(cmd, args, toComplete)
}
