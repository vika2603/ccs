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
ccs new home --from ~/.claude  # adopt the existing setup as profile "home"
ccs new work                   # an empty profile linked to the shared assets
ccs new work2 --from work      # clone a profile
ccs use work                   # make "work" the active profile
claude                         # runs against the active profile
ccs ls                         # profiles and the account each one uses
ccs status                     # account, path, and shared-field state
ccs doctor --fix               # check and repair the ~/.ccs tree
```

`ccs use` applies to every shell and GUI app at once, because the shim reads
the active profile each time `claude` starts; `ccs use --none` falls back to
`~/.claude`. To run claude once without switching, use `ccs work`, or
`ccs work -- <claude args>`; `ccs -- <claude args>` uses the active profile.

Per-profile env vars (`ccs env set <profile> KEY=VAL`) are injected only into
the `claude` process. They do **not** appear in `env` / `printenv` in your
regular shell.

Shared assets are edited once for all profiles. `ccs field fork <field>` gives
the active profile its own copy, and `ccs field share <field>` pushes it back.

`ccs backup` and `ccs restore` move the whole `~/.ccs` tree, with OAuth tokens
encrypted by a passphrase.

`ccs --help` lists everything else.
