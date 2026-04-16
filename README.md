# ccs

Run multiple Claude Code profiles from one machine. Each profile keeps its
own credentials and history; the skills, commands, agents, and `CLAUDE.md`
you want everywhere are shared by default, and edits propagate across all
profiles until you fork them.

## Install

```sh
curl -fsSL https://github.com/vika2603/ccs/releases/latest/download/install.sh | sh
```

Add to `~/.zshrc` or `~/.bashrc`:

```sh
eval "$(ccs shell-init)"
```

## Usage

```sh
ccs init
ccs import ~/.claude home    # adopt the existing setup as profile "home"
ccs new work                 # a fresh empty profile
ccs use work                 # switch this shell to "work"
claude                       # runs against the "work" profile
```

`ccs --help` lists everything else.
