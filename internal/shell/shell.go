package shell

import (
	"path/filepath"
	"strings"
)

type Shell int

const (
	Zsh Shell = iota
	Bash
)

func Detect(shellPath string) Shell {
	base := filepath.Base(shellPath)
	if strings.Contains(base, "bash") {
		return Bash
	}
	return Zsh
}

func Render(s Shell) string {
	switch s {
	case Bash:
		return bashSnippet
	default:
		return zshSnippet
	}
}

const zshSnippet = `[[ $- == *i* ]] || return 0
[[ -n ${_CCS_SHELL_INIT_DONE-} ]] && return 0
_CCS_SHELL_INIT_DONE=1

ccs() {
  if [ "$1" = use ]; then
    shift
    eval "$(command ccs __shell_use "$@")"
  else
    command ccs "$@"
  fi
}

_ccs_hook() {
  [ -n "$CCS_DISABLE_HOOK" ] && return
  if [ -n "$CLAUDE_CONFIG_DIR" ] && [ -z "$CCS_MANAGED_CCD" ]; then
    return
  fi
  local want
  want="$(command ccs path 2>/dev/null || true)"
  if [ -n "$want" ]; then
    if [ "$CLAUDE_CONFIG_DIR" != "$want" ]; then
      export CLAUDE_CONFIG_DIR="$want"
      export CCS_MANAGED_CCD=1
    fi
  else
    if [ -n "$CCS_MANAGED_CCD" ]; then
      unset CLAUDE_CONFIG_DIR CCS_MANAGED_CCD
    fi
  fi
}

typeset -ga precmd_functions
(( ${precmd_functions[(Ie)_ccs_hook]} )) || precmd_functions+=(_ccs_hook)
`

const bashSnippet = `case $- in *i*) ;; *) return 0 ;; esac
[ -n "${_CCS_SHELL_INIT_DONE-}" ] && return 0
_CCS_SHELL_INIT_DONE=1

ccs() {
  if [ "$1" = use ]; then
    shift
    eval "$(command ccs __shell_use "$@")"
  else
    command ccs "$@"
  fi
}

_ccs_hook() {
  [ -n "$CCS_DISABLE_HOOK" ] && return
  if [ -n "$CLAUDE_CONFIG_DIR" ] && [ -z "$CCS_MANAGED_CCD" ]; then
    return
  fi
  local want
  want="$(command ccs path 2>/dev/null || true)"
  if [ -n "$want" ]; then
    if [ "$CLAUDE_CONFIG_DIR" != "$want" ]; then
      export CLAUDE_CONFIG_DIR="$want"
      export CCS_MANAGED_CCD=1
    fi
  else
    if [ -n "$CCS_MANAGED_CCD" ]; then
      unset CLAUDE_CONFIG_DIR CCS_MANAGED_CCD
    fi
  fi
}

if declare -p PROMPT_COMMAND 2>/dev/null | grep -q '^declare -a'; then
  _ccs_already=
  for _c in "${PROMPT_COMMAND[@]}"; do
    [ "$_c" = _ccs_hook ] && _ccs_already=1 && break
  done
  [ -z "$_ccs_already" ] && PROMPT_COMMAND=(_ccs_hook "${PROMPT_COMMAND[@]}")
  unset _c _ccs_already
else
  case ";${PROMPT_COMMAND-};" in
    *';_ccs_hook;'*) ;;
    *) PROMPT_COMMAND="_ccs_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}" ;;
  esac
fi
`
