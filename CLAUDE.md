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
  run/<name>.lock               flock serializing profile.Sync of a login profile
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
- `profile.Sync` runs before every launch of a login profile, under `run/<name>.lock`:
  - each `~/.claude` entry missing from the account directory is symlinked to `~/.claude`;
  - an isolated entry that is still such a link, or is missing, is copied in. The copy is staged in a temporary sibling and renamed into place, so an interrupted copy leaves nothing behind;
  - links to entries removed from `~/.claude` are dropped.
  - Entries in `identity` (`internal/profile/profile.go`) are never linked, and existing links to them are removed: `.claude.json` and `.config.json` (global state with `oauthAccount`), `backups`, `.credentials.json`, and the organization policy, remote settings, and connector caches Claude Code keeps per account. Add a name there when Claude Code starts keeping another per-account file in the config directory.
  - Real files already in the account directory are never replaced, so nothing an account wrote is lost; sharing an entry again means deleting the account's copy.
- `[settings]` is written to `run/<name>.settings.json` and passed as `claude --settings <file> ...`. It must come before any claude subcommand (`claude mcp list --settings x` is rejected). A file is used instead of inline JSON so tokens never appear on a command line. Claude Code applies `--settings` above user settings, merges `env` by variable name, and lets settings `env` win over the process environment.
- `profile.Remove` does not parse the profile file: it deletes the account directory and its Keychain item whenever the directory exists, so an invalid profile or one edited to drop `login` leaves nothing behind.
- `profile.Account` derives the label shown by `ls` from `ANTHROPIC_BASE_URL` in `[settings]` or the settings.json the profile runs with, otherwise the `oauthAccount` in the profile's `.claude.json`; it never reads tokens.

## Launching (`cmd/ccs/root.go`, `internal/launch`)

- `ccs [name] [-- args]` launches the named profile, or the active one. A `--` after the name is dropped before the args reach claude, because flag parsing stops at the name.
- Every profile launch goes through `launch.Env`: it sets `CCS_PROFILE=<name>`, sets or removes `CLAUDE_CONFIG_DIR`, and removes inherited `ANTHROPIC_*` and `CLAUDE_CODE_OAUTH_TOKEN`. Claude Code exports settings `env` to its children, so without the removal a profile started inside another profile's session would run on the outer gateway token instead of its own login. Endpoint, credentials, and models therefore come only from settings (`[settings.env]` or `~/.claude/settings.json`), never from the shell.
- `ccs -- args` (leading `--`, `cmd.ArgsLenAtDash() == 0`) and the shim behave like plain `claude args`, in this order: inside a ccs-launched session (`CCS_PROFILE` set and `CLAUDE_CONFIG_DIR` still matching that profile) the same profile is launched again; otherwise a `CLAUDE_CONFIG_DIR` set by the caller is used as is, with the environment untouched; otherwise the active profile applies. This keeps a hook, the Bash tool, or a wrapper that resolves `claude` back to the shim on the profile its session was started with.
- `launch.Resolve` skips `~/.ccs/bin` when resolving `claude`, so the shim never execs itself.
- The hidden `__shim_exec claude [args]` command is the contract with the generated shim script (`shimScript` in `cmd/ccs/commands.go`); existing shims call it until the next `ccs init`, so keep its argument shape stable. `ccs init` replaces a symlink at the shim path instead of writing through it.

## Testing conventions

- Package tests use `t.TempDir()` as fake `$HOME` and construct `layout.Paths` via `layout.New(tmp)`; don't rely on the real `~/.ccs` or `~/.claude`.
- Platform-specific files use build tags (`//go:build darwin` / `//go:build linux`) and `_darwin` / `_linux` filename suffixes. Only macOS and Linux are supported.
- Test files are exempt from errcheck in `.golangci.yml`; production code must handle or explicitly join write/close errors.
