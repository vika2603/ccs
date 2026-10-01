# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build, test, lint

```sh
make build                     # -> ./bin/ccs
make test                      # go test -race ./...
make lint                      # go vet + golangci-lint (config: .golangci.yml)
make fmt                       # gofmt + goimports via golangci-lint
make check                     # lint + test; what CI runs
go test ./internal/profile/... # single package
go test -run TestLoginProfileLinksClaudeDir ./test/e2e   # single e2e test (builds its own binary)
```

The e2e tests in `test/e2e/e2e_test.go` build the binary, run it against `HOME=<tempdir>`, and put a fake `claude` that prints `CLAUDE_CONFIG_DIR` and its arguments on `PATH`. They never touch the real `~/.ccs` or `~/.claude`, and they clear `CLAUDE_CONFIG_DIR` because the test process may itself run under a ccs-managed claude.

CI (`.github/workflows/ci.yml`) runs tidy check, vet, and race tests on Linux and macOS, plus golangci-lint and govulncheck. Release uses goreleaser v2 (`.goreleaser.yaml`) triggered by pushing a `v*` tag; `install.sh` consumes those GitHub release artifacts. The Go version is taken from `go.mod` everywhere.

## What ccs is

ccs runs Claude Code with several accounts or API gateways on top of one `~/.claude`. `~/.claude` is the `default` profile and the single source of skills, commands, agents, hooks, `CLAUDE.md`, and settings. Every other profile is a TOML file that only describes what differs.

## Disk layout (`internal/layout`)

```
~/.claude/                      default profile; source for all others (not owned by ccs)
~/.claude.json                  default profile's global state (Claude Code keeps it next to ~/.claude)
~/.ccs/
  profiles/<name>.toml          a profile (0600; may hold API tokens)
  accounts/<name>/              CLAUDE_CONFIG_DIR of a login profile
  run/<name>.settings.json      [settings] of a profile, written before each launch (0600)
  state/active                  active profile name (replaced atomically; missing means default)
  bin/claude                    shim written by `ccs init`
```

Profile names: `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`. `default` and the subcommand names (`init`, `new`, `edit`, `ls`, `use`, `rm`, `help`, `completion`) are reserved, because `ccs <name>` would otherwise be shadowed (`internal/layout/active.go`).

## Profiles (`internal/profile`)

```toml
login = false        # true: own config directory for a separate OAuth login
isolate = []         # login only: ~/.claude entries this profile keeps to itself
[settings]           # passed to claude --settings; any Claude Code setting
```

- Without `login`, Claude Code runs on `~/.claude` itself (`CLAUDE_CONFIG_DIR` removed from the environment). This suits API gateways: `ANTHROPIC_AUTH_TOKEN`/`ANTHROPIC_API_KEY` take priority over the OAuth login stored for `~/.claude`.
- With `login`, `CLAUDE_CONFIG_DIR` is `~/.ccs/accounts/<name>`. Claude Code keeps one login per config directory (the macOS Keychain service name is derived from the directory path, see `internal/creds/service.go`; `ccs rm` deletes that item), which is why a second OAuth account needs its own directory.
- `profile.Sync` runs before every launch of a login profile: each `~/.claude` entry missing from the account directory is symlinked to `~/.claude`; an isolated entry that is still such a link is replaced by a copy, and a missing isolated entry is copied in, and links to entries removed from `~/.claude` are dropped. `.claude.json`, `.credentials.json`, and `backups` (backups of `.claude.json`) are never linked. Real files already in the account directory are never replaced, so nothing an account wrote is lost; sharing an entry again means deleting the account's copy.
- `[settings]` is written to `run/<name>.settings.json` and passed as `claude --settings <file> ...`. It must come before any claude subcommand (`claude mcp list --settings x` is rejected). A file is used instead of inline JSON so tokens never appear on a command line. Claude Code applies `--settings` above user settings, merges `env` by variable name, and lets settings `env` win over the process environment.
- `profile.Account` derives the label shown by `ls` from `ANTHROPIC_BASE_URL` in `[settings]` or the settings.json the profile runs with, otherwise the `oauthAccount` in the profile's `.claude.json`; it never reads tokens.

## Launching (`cmd/ccs/root.go`, `internal/launch`)

- `ccs [name] [-- args]` launches the named profile, or the active one. A `--` after the name is dropped before the args reach claude, because flag parsing stops at the name.
- `ccs -- args` (leading `--`, `cmd.ArgsLenAtDash() == 0`) and the shim behave like plain `claude args`: a `CLAUDE_CONFIG_DIR` already set by the caller is used as is, otherwise the active profile applies. Honoring the caller's directory keeps a wrapper that resolves `claude` back to the shim on the profile it was started with.
- `launch.Resolve` skips `~/.ccs/bin` when resolving `claude`, so the shim never execs itself.
- The hidden `__shim_exec claude [args]` command is the contract with the generated shim script (`shimScript` in `cmd/ccs/commands.go`); existing shims call it until the next `ccs init`, so keep its argument shape stable. `ccs init` replaces a symlink at the shim path instead of writing through it.

## Testing conventions

- Package tests use `t.TempDir()` as fake `$HOME` and construct `layout.Paths` via `layout.New(tmp)`; don't rely on the real `~/.ccs` or `~/.claude`.
- Platform-specific files use build tags (`//go:build darwin` / `//go:build linux`) and `_darwin` / `_linux` filename suffixes. Only macOS and Linux are supported.
- Test files are exempt from errcheck in `.golangci.yml`; production code must handle or explicitly join write/close errors.
