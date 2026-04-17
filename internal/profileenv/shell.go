package profileenv

import (
	"fmt"
	"strings"
)

// Action describes one shell-eval block to emit.
//
// Profile env vars (Set) are no longer exported into the user's shell - they
// only reach the claude process via the PATH shim (~/.ccs/bin/claude) which
// routes through `ccs run`. The Set field stays on the struct because callers
// still hand it in, but Render ignores it.
//
// The produced block:
//  1. Unsets every env var named in $CCS_MANAGED_VARS. This is migration-only
//     cleanup for shells that still carry vars exported by older ccs versions;
//     new profile activations don't add anything to CCS_MANAGED_VARS.
//  2. If Sig != "", exports CCS_ENV_SIG so the prompt hook can fast-path when
//     nothing has changed.
//  3. Exports CLAUDE_CONFIG_DIR + CCS_MANAGED_CCD when ConfigDir != "".
type Action struct {
	Set       map[string]string
	ConfigDir string
	Sig       string
}

// Render emits the shell eval block for Action.
func Render(a Action) string {
	var b strings.Builder
	writeUnsetManagedLoop(&b)

	if a.Sig != "" {
		fmt.Fprintf(&b, "export CCS_ENV_SIG=%s\n", ShellQuote(a.Sig))
	}
	if a.ConfigDir != "" {
		fmt.Fprintf(&b, "export CLAUDE_CONFIG_DIR=%s\n", ShellQuote(a.ConfigDir))
		b.WriteString("export CCS_MANAGED_CCD=1\n")
	}
	return b.String()
}

// RenderClearAll emits the eval block for an explicit clear: unset every
// ccs-managed env var, then drop CCS_MANAGED_VARS, CCS_ENV_SIG, and - since
// the user explicitly asked to clear - CLAUDE_CONFIG_DIR / CCS_MANAGED_CCD
// unconditionally.
func RenderClearAll() string {
	var b strings.Builder
	writeUnsetManagedLoop(&b)
	b.WriteString("unset CCS_MANAGED_VARS CCS_ENV_SIG CLAUDE_CONFIG_DIR CCS_MANAGED_CCD\n")
	return b.String()
}

// RenderClearManaged emits the eval block used by the prompt hook when the
// active profile becomes empty: unset managed env vars, clear CCS_MANAGED_VARS
// and CCS_ENV_SIG, and only drop CLAUDE_CONFIG_DIR if it was set by ccs
// (CCS_MANAGED_CCD).
func RenderClearManaged() string {
	var b strings.Builder
	writeUnsetManagedLoop(&b)
	b.WriteString("unset CCS_MANAGED_VARS CCS_ENV_SIG\n")
	b.WriteString("[ -n \"${CCS_MANAGED_CCD-}\" ] && unset CLAUDE_CONFIG_DIR CCS_MANAGED_CCD\n")
	return b.String()
}

// writeUnsetManagedLoop writes a POSIX-portable loop that unsets every name
// listed in CCS_MANAGED_VARS (space-separated). Uses eval so the expansion is
// re-parsed: this works in both bash (default word-splitting) and zsh (which
// does not word-split bare $foo by default).
func writeUnsetManagedLoop(b *strings.Builder) {
	b.WriteString("if [ -n \"${CCS_MANAGED_VARS-}\" ]; then eval \"for _v in $CCS_MANAGED_VARS; do unset \\\"\\$_v\\\"; done\"; unset _v; fi\n")
}

// ShellQuote wraps s in single quotes with proper escaping so that `eval` in
// POSIX sh / bash / zsh reads the value verbatim.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
