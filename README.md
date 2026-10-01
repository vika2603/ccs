# ccs

Run Claude Code with several accounts or API gateways on one machine, all on
top of a single `~/.claude`. Skills, commands, agents, hooks, `CLAUDE.md`,
and settings live in `~/.claude` as usual; a profile only adds what differs.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/vika2603/ccs/main/install.sh | sh
ccs init
```

`ccs init` installs a `claude` shim at `~/.ccs/bin/claude` that starts
Claude Code with the active profile. Put `~/.ccs/bin` first on `PATH` in
`~/.zprofile` (not just `~/.zshrc`, so GUI apps like VS Code and JetBrains see
it too):

```sh
export PATH="$HOME/.ccs/bin:$PATH"
```

Tab completion: `source <(ccs completion zsh)` (or `bash`).

## Profiles

`default` is `~/.claude` itself with its own login. Every other profile is a
file in `~/.ccs/profiles/<name>.toml`, created with `ccs new` and changed with
`ccs edit`.

An API gateway or API key runs on `~/.claude` and only adds settings:

```toml
# ccs new gw
[settings.env]
ANTHROPIC_BASE_URL = "https://gateway.example.com"
ANTHROPIC_AUTH_TOKEN = "..."
```

A second OAuth account needs its own config directory, because Claude Code
keeps one login per directory:

```toml
# ccs new work --login, then `ccs work` and /login once
login = true
isolate = ["projects", "history.jsonl"]   # optional
```

Everything in `~/.claude` is linked into the account directory
(`~/.ccs/accounts/<name>`) except the account identity and the names in
`isolate`, which the profile keeps to itself. New `~/.claude` entries are
linked on the next start.

`[settings]` is passed to `claude --settings`, so it applies on top of
`~/.claude/settings.json` for that profile only, and its `env` wins over the
same variables in `~/.claude/settings.json`.

ccs ignores `ANTHROPIC_*` variables and `CLAUDE_CODE_OAUTH_TOKEN` from the
shell, so a profile started from inside another profile's session uses its
own login or gateway. Put API endpoints and keys in `[settings.env]` (or in
`~/.claude/settings.json` for `default`). A `claude` started inside a ccs
session, for example by a hook, runs with that session's profile.

## Usage

```sh
ccs ls                  # profiles, their kind and account; * marks the active one
ccs use work            # `claude` now runs as work, in every shell and GUI app
ccs use default         # back to plain ~/.claude
ccs gw                  # run claude once with gw, without switching
ccs gw -- -c            # same, passing -c to claude
ccs -- -c               # pass -c to claude with the active profile
ccs rm work             # remove a profile; for a login profile also its directory and stored login
```

`ccs --help` lists everything.
