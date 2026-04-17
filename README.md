# ccs

Run multiple Claude Code profiles from one machine. Each profile keeps its
own credentials and history; the skills, commands, agents, and `CLAUDE.md`
you want everywhere are shared by default, and edits propagate across all
profiles until you fork them.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/vika2603/ccs/main/install.sh | sh
```

Add to `~/.zshrc` or `~/.bashrc`:

```sh
eval "$(ccs shell-init)"
```

Install the `claude` shim so per-profile env vars reach the `claude`
process without leaking into your daily shell:

```sh
ccs install-shim
```

Add `~/.ccs/bin` to `PATH` in `~/.zprofile` (not just `~/.zshrc`, so GUI
apps like VS Code and JetBrains see it too):

```sh
export PATH="$HOME/.ccs/bin:$PATH"
```

## Usage

```sh
ccs init
ccs import ~/.claude home    # adopt the existing setup as profile "home"
ccs new work                 # a fresh empty profile
ccs use work                 # switch this shell to "work"
claude                       # runs against the "work" profile
```

Per-profile env vars (set via `ccs env set <profile> KEY=VAL`) are injected
only into the `claude` process when the shim runs it. They do **not** appear
in `env` / `printenv` in your regular shell.

`ccs --help` lists everything else.
