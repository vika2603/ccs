# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build, test, lint

```sh
make build                    # -> ./bin/ccs
make test                     # go test ./...
make lint                     # go vet ./...
go test ./internal/fields/... # single package
go test -run TestFullFlow ./test/e2e   # single e2e test (builds its own binary)
```

The e2e test in `test/e2e/e2e_test.go` shells out to `go build ./cmd/ccs` and runs it against a `HOME=<tempdir>`, so e2e runs require a working Go toolchain and are hermetic — they never touch the real `~/.ccs`.

Release uses goreleaser (`.goreleaser.yaml`) triggered by pushing a `v*` tag; `install.sh` consumes those GitHub release artifacts.

## What ccs is

A profile switcher for Claude Code. One machine, many profiles. Each profile has its own credentials, history, and account identity, but **shared assets** (`skills`, `commands`, `agents`, `plugins`, `CLAUDE.md`, `settings.json`) are symlinks into `~/.ccs/shared/` so editing them from any profile propagates to all of them — until that profile **forks** the asset into a real copy.

The bare `ccs` command and `ccs <profile>` both `syscall.Exec` into `claude` with `CLAUDE_CONFIG_DIR` pointed at the profile's directory (`cmd/ccs/run_cmd.go`).

## Disk layout (`internal/layout/layout.go`)

```
~/.ccs/
  config.toml          # shared/isolated classification + export excludes + launch.command
  state/active         # name of active profile (flock-guarded; see internal/state)
  shared/<field>       # real files/dirs that profiles symlink to
  profiles/<name>/     # CLAUDE_CONFIG_DIR for that profile (mix of symlinks + real files)
  env/<name>.toml      # per-profile env vars (0600); applied via shell hook
```

Profile names: `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`, with `default|shared|state|config` reserved (`internal/state/state.go`).

## Field classification (`internal/fields`, `internal/config/defaults.go`)

The single most important concept. Every top-level entry under a profile directory is classified in `config.toml`:

- **Shared**: symlinked into `~/.ccs/shared/<field>`. Default: `skills`, `commands`, `agents`, `plugins`, `CLAUDE.md`, `settings.json`.
- **Isolated**: real file/dir living inside the profile. Default includes `.claude.json` (holds account identity — `oauthAccount`, `userID`, onboarding flags), `.credentials.json` (Linux only), `history.jsonl`, `projects`, `sessions`, `todos`, `statsig`, etc.
- **Excluded from export**: `cache`, `chrome`, `paste-cache`, `stats-cache.json`.

When adding or changing a default, update `internal/config/defaults.go` AND think about whether the entry is (a) safe to share across profiles and (b) regeneratable if excluded from export. `.claude.json` must stay Isolated — it pairs with the per-profile OAuth token in the credential store; sharing it would cross-contaminate identities.

Entries not in either list are treated as isolated at runtime but reported by `ccs doctor` as `unclassified-entry` and by `ccs classify <name> <shared|isolated>` to pin them.

Kind (file vs dir) is inferred from name (`kindOverrides` in `fields.go` handles dotfiles like `.credentials.json` that have no distinguishing extension).

## Fork / share / relink lifecycle (`internal/fields/ops.go`)

- `ccs fork <field> [profile]` — replaces the profile's symlink with a real copy of `shared/<field>` so edits from this profile are local.
- `ccs share <field> [profile]` — pushes the forked copy back into `shared/`, prompting the TUI conflict picker if `shared/<field>` is non-empty, then re-creates the symlink. The share path is the only write path into `shared/` other than `init`.
- `ccs relink <field> [profile]` — recreates a missing symlink to `shared/<field>`. Refuses to overwrite a real copy (must `share` first).
- `ccs status` — reports each shared field as `linked | forked | missing`.

## Credentials (`internal/creds`)

Storage is per-platform behind `Store`:

- `keychain_darwin.go` — macOS Keychain via `security`, one service per profile, name derived in `service.go` as `Claude Code-credentials-<sha8(abs_profile_path)>`. The service name for `~/.claude` itself is the bare `Claude Code-credentials` (matches vanilla Claude Code so `import` can adopt it in place).
- `file_linux.go` — `<profile>/.credentials.json`, mode 0600. This is why `.credentials.json` is Isolated in defaults.

`creds.Migrate` (used by `ccs mv`) is write-new → verify-roundtrip → delete-old; if verification fails it keeps both rather than risk losing a token.

## Env vars per profile (`internal/profileenv`, shell hook)

`ccs env set/unset/ls/edit/get <profile>` writes `~/.ccs/env/<profile>.toml` (0600). Names must match `^[A-Za-z_][A-Za-z0-9_]*$`. Values are masked in `env ls` unless `--show-values`.

The shell integration (`internal/shell/shell.go`) installs a precmd/`PROMPT_COMMAND` hook that invokes the hidden `ccs __shell_hook` on every prompt. The hook compares `$CCS_ENV_SIG` (set from profile name + env file mtime) against the current signature and emits `export`/`unset` lines only when they diverge. Invariant: if the user has `CLAUDE_CONFIG_DIR` set and `CCS_MANAGED_CCD` is unset, the hook does nothing — never overwrite a user's own CCD. When flipping active profile via `ccs use`, the shell wrapper in `shell.go` rewrites the arg to the hidden `__shell_use` subcommand and `eval`s its output, so env mutations happen in the calling shell.

## Export / import / restore (`internal/archive`, `internal/fields/preset.go`, `internal/tui/picker`)

Archive is a gzipped tar with `manifest.json` at root:

- `profile/<entry>` — resolved (symlinks dereferenced) copies of selected profile entries.
- `shared/<field>` — for each selected shared entry that was still symlinked in the profile, the real content from `~/.ccs/shared/`.
- `credentials.json.age` — only if `--with-credentials`; encrypted via `filippo.io/age` passphrase recipient (`internal/archive/age.go`).

Export presets live in `internal/fields/preset.go`: `default` (shared + `.claude.json` subset), `with-credentials`, `full` (adds isolated runtime data: `projects`, `todos`, `history.jsonl`). `-i` runs a bubbletea picker seeded from the preset. Scope-of-protection notice is printed to stderr in `cmd/ccs/export_cmd.go` — **do not remove it**; the tarball itself and `manifest.json` are plaintext and only the OAuth token is encrypted.

Restore refuses cross-platform archives (`m.SourcePlatform != runtime.GOOS`) because the credential store shape differs between macOS and Linux.

## Command wiring

All subcommands registered in `cmd/ccs/root.go`; each `new*Cmd()` lives in its own file. `manager()` in `cmd/ccs/profile_cmds.go` is the canonical way to get `(profile.Manager, layout.Paths)` — it loads `config.toml`, builds the field registry, and attaches the platform `creds.Store`. Prefer it over constructing the pieces directly so `WithCreds` stays consistent.

Hidden commands (`__shell_use`, `__shell_hook`) are contracts with the generated shell snippet in `internal/shell/shell.go` — if you change their stdout format, update the matching `testdata/*.golden` and make sure both zsh and bash snippets still parse it.

## Testing conventions

- Package tests use `t.TempDir()` as fake `$HOME` and construct `layout.Paths` via `layout.New(tmp)`; don't rely on the real `~/.ccs`.
- Platform-specific files use build tags (`//go:build darwin` / `//go:build linux`) and have `_darwin` / `_linux` suffixes in filenames; the `_others.go` variants cover unsupported platforms so the package still builds.
- Shell snippet tests compare against `internal/shell/testdata/{bash,zsh}.golden`. Regenerate golden files deliberately — the snippet is shipped verbatim to users.
- `test/e2e/e2e_test.go` is the cross-cutting smoke: init → import → new → use → fork → status → export → restore → doctor.
