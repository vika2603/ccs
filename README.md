# ccs

Run multiple Claude Code profiles from one machine. Each profile keeps its
own credentials and history; the skills, commands, agents, and `CLAUDE.md`
you want everywhere are shared by default, and edits propagate across all
profiles until you fork them.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/vika2603/ccs/main/install.sh | sh
```

Run `ccs init` once. It creates `~/.ccs` and installs a `claude` shim at
`~/.ccs/bin/claude` that starts Claude Code with the active profile. Put
`~/.ccs/bin` first on `PATH` in `~/.zprofile` (not just `~/.zshrc`, so GUI
apps like VS Code and JetBrains see it too):

```sh
export PATH="$HOME/.ccs/bin:$PATH"
```

Tab completion: `source <(ccs completion zsh)` (or `bash`).

## Usage

```sh
ccs adopt ~/.claude home     # adopt the existing setup as profile "home"
ccs new work                 # a fresh empty profile
ccs use work                 # make "work" the active profile
claude                       # runs against the "work" profile
ccs ls                       # profiles and the account each one uses
ccs doctor --fix             # check and repair the ~/.ccs tree
```

`ccs use` applies to every shell and GUI app at once, because the shim reads
the active profile each time `claude` starts. To run one profile without
switching, use `ccs work` or `ccs run work -- <command>`.

Per-profile env vars (set via `ccs env set <profile> KEY=VAL`) are injected
only into the `claude` process when the shim or `ccs run` starts it. They do **not** appear
in `env` / `printenv` in your regular shell.

`ccs --help` lists everything else.
