# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build, test, lint

```sh
make build                    # -> ./bin/ccs
make test                     # go test -race ./...
make lint                     # go vet + golangci-lint (config: .golangci.yml)
make fmt                      # gofmt + goimports via golangci-lint
make check                    # lint + test; what CI runs
go test ./internal/fields/... # single package
go test -run TestFullFlow ./test/e2e   # single e2e test (builds its own binary)
```

The e2e test in `test/e2e/e2e_test.go` shells out to `go build ./cmd/ccs` and runs it against a `HOME=<tempdir>`, so e2e runs require a working Go toolchain and are hermetic — they never touch the real `~/.ccs`.

CI (`.github/workflows/ci.yml`) runs tidy check, vet, and race tests on Linux and macOS, plus golangci-lint and govulncheck. Release uses goreleaser v2 (`.goreleaser.yaml`) triggered by pushing a `v*` tag; `install.sh` consumes those GitHub release artifacts. The Go version is taken from `go.mod` everywhere.

## What ccs is

A profile switcher for Claude Code. One machine, many profiles. Each profile has its own credentials, history, and account identity, but **shared assets** (by default `skills`, `commands`, `agents`, `CLAUDE.md`, `settings.json`) are symlinks into `~/.ccs/shared/` so editing them from any profile propagates to all of them — until that profile **forks** the asset into a real copy.

The bare `ccs` command and `ccs <profile>` both `syscall.Exec` into `claude` with `CLAUDE_CONFIG_DIR` pointed at the profile's directory (`app.launch` in `cmd/ccs/run_cmd.go`).

## Package map

- `cmd/ccs` — cobra wiring only. `app.go` loads paths, config, registry, credential store, and profile manager once per command.
- `internal/layout` — `~/.ccs` paths, profile name validation, and the flock-guarded active-profile pointer (`Paths.Active/SetActive/ClearActive`).
- `internal/config` — `config.toml` load/save and shipped defaults.
- `internal/fields` — field registry plus fork/share/relink/adopt logic. Has no UI dependency; callers pass a `ConflictFunc`.
- `internal/profile` — profile create/remove/rename/clone, and the account label shown by `ls`/`status`.
- `internal/creds` — per-platform credential store and keychain service enumeration.
- `internal/archive` — backup tarballs; extraction goes through `os.Root`.
- `internal/doctor` — consistency checks; `ccs doctor --fix` repairs the findings with a safe remedy. Orphan keychain entries are never deleted automatically because a service name only carries a hash of its directory.
- `internal/profileenv` — per-profile env files and the env/argv used to launch claude.
- `internal/tui` — the share/adopt conflict prompt.
- `internal/fsutil` — copy, atomic write, symlink, and emptiness helpers shared by the packages above.

## Disk layout (`internal/layout/layout.go`)

```
~/.ccs/
  config.toml          # shared/isolated classification + backup excludes + launch.command
  state/active         # name of active profile (flock-guarded; see internal/layout/active.go)
  shared/<field>       # real files/dirs that profiles symlink to
  profiles/<name>/     # CLAUDE_CONFIG_DIR for that profile (mix of symlinks + real files)
  bin/claude           # shim (re)written by every `ccs init`
  env/<name>.toml      # per-profile env vars (0600); injected only into the claude process
```

Profile names: `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`, with `default|shared|state|config` reserved (`internal/layout/active.go`).

## Field classification (`internal/fields`, `internal/config/defaults.go`)

The single most important concept. Every top-level entry under a profile directory is classified in `config.toml`:

- **Shared**: symlinked into `~/.ccs/shared/<field>`. Default: `skills`, `commands`, `agents`, `CLAUDE.md`, `settings.json`.
- **Isolated**: real file/dir living inside the profile. Default includes `.claude.json` (holds account identity — `oauthAccount`, `userID`, onboarding flags), `.credentials.json` (Linux only), `plugins`, `history.jsonl`, `projects`, `sessions`, `todos`, `statsig`, etc.
- **Excluded from backup** (`[export] exclude`): `cache`, `plugin`, `chrome`, `paste-cache`, `stats-cache.json`. Excluded names are also treated as isolated.

When adding or changing a default, update `internal/config/defaults.go` AND think about whether the entry is (a) safe to share across profiles and (b) regeneratable if excluded from backup. `.claude.json` must stay Isolated — it pairs with the per-profile OAuth token in the credential store; sharing it would cross-contaminate identities.

Entries not in either list are treated as isolated at runtime but reported by `ccs doctor` as `unclassified-entry` and pinned with `ccs field classify <name> <shared|isolated>`.

Kind (file vs dir) is inferred from name (`kindOverrides` in `fields.go` handles dotfiles like `.credentials.json` that have no distinguishing extension).

## Fork / share / relink lifecycle (`internal/fields/ops.go`)

- `ccs field fork <field> [profile]` — replaces the profile's symlink with a real copy of `shared/<field>` so edits from this profile are local.
- `ccs field share <field> [profile]` — pushes the forked copy back into `shared/`, prompting the conflict prompt (`internal/tui`) if `shared/<field>` is non-empty, then re-creates the symlink. Besides `share`, adopting (`new --from <dir>`) and `restore` copy content into `shared/`; `init`, `new`, and `doctor --fix` only create missing empty targets.
- `ccs doctor --fix` — recreates missing symlinks to `shared/<field>` via `Ops.Relink`, which refuses to overwrite a real copy (must `field share` first). Profiles with no shared link at all (`ccs new --blank`) are left alone.
- `ccs status` — reports each shared field as `linked | forked | missing`.

## Credentials (`internal/creds`)

Storage is per-platform behind `Store`:

- `keychain_darwin.go` — macOS Keychain via `security`, one service per profile, name derived in `service.go` as `Claude Code-credentials-<sha8(abs_profile_path)>`. The service name for `~/.claude` itself is the bare `Claude Code-credentials` (matches vanilla Claude Code so `new --from ~/.claude` can take it over in place).
- `file_linux.go` — `<profile>/.credentials.json`, mode 0600. This is why `.credentials.json` is Isolated in defaults.

`creds.Migrate` (used by `ccs mv`) is write-new → verify-roundtrip → delete-old; if verification fails it keeps both rather than risk losing a token.

## Active profile, shim, and env vars (`cmd/ccs/shim_cmds.go`, `internal/profileenv`)

`ccs init` writes `~/.ccs/bin/claude`, which calls the hidden `ccs __shim_exec`. The shim reads `state/active` on every start and execs claude with `CLAUDE_CONFIG_DIR` and the profile's env vars (`profileenv.BuildEnv`), so `ccs use` takes effect in every shell and GUI app without any shell integration. There is no shell integration besides `ccs completion <shell>`.

If `CLAUDE_CONFIG_DIR` is already set, the shim passes the environment through unchanged. This keeps `ccs <profile>` with a wrapping `launch.command` (e.g. `["caffeinate", "-is", "claude"]`) on the chosen profile when the wrapper resolves `claude` back to the shim.

The root command is `ccs [profile] [-- claude-args]`. A leading `--` (`ccs -- -c`) names no profile and behaves like `claude -c` through the shim: an explicit `CLAUDE_CONFIG_DIR` wins, then the active profile, then plain claude (`cmd.ArgsLenAtDash() == 0` in `cmd/ccs/root.go`).

`ccs env set/unset/ls/edit <profile>` writes `~/.ccs/env/<profile>.toml` (0600). Names must match `^[A-Za-z_][A-Za-z0-9_]*$`. Values are masked in `env ls` unless `--show-values`. Profile env vars never reach the user's shell.

## Creating profiles, backup, restore (`cmd/ccs/profile_cmds.go`, `cmd/ccs/adopt_cmd.go`, `internal/archive`)

`ccs new <name>` creates an empty profile; `--from <profile>` clones one; `--from <dir>` adopts an existing `.claude`-style directory and links every configured shared field, as a plain `new` does. A `--from` value containing a path separator is always a directory; otherwise an existing profile of that name wins over a same-named directory.

`ccs backup`/`ccs restore` move the whole `~/.ccs` tree. The backup archive uses `backup-manifest.json`, keeps symlinks into `~/.ccs` as relative links, and stores all profiles' tokens in one age-encrypted bundle. `cmd/ccs/backup_cmd.go` prints a scope-of-protection notice to stderr — **do not remove it**; the tarball is plaintext except for the encrypted tokens.

Extraction (`archive.extract`) treats archives as untrusted input: it rejects entry names outside the destination and any entry placed beneath a symlink, checks each symlink target from the directory it is created in, and writes through `os.Root`. `restore` validates the profile names in the credentials bundle before using them as paths.

Restore refuses cross-platform archives (`m.SourcePlatform != runtime.GOOS`) because the credential store shape differs between macOS and Linux.

## Command wiring

All subcommands registered in `cmd/ccs/root.go`; each `new*Cmd()` lives in its own file. `loadApp()` in `cmd/ccs/app.go` is the only way commands obtain their dependencies: it embeds `layout.Paths` and carries the loaded config, field registry, credential store, and profile manager. Use `a.profileOrActive(name)` for "explicit profile or the active one" arguments.

The hidden `__shim_exec` command is a contract with the shim script generated in `cmd/ccs/shim_cmds.go`; already-installed shims call it until the next `ccs init`, so keep its argument shape stable.

## Testing conventions

- Package tests use `t.TempDir()` as fake `$HOME` and construct `layout.Paths` via `layout.New(tmp)`; don't rely on the real `~/.ccs`.
- Platform-specific files use build tags (`//go:build darwin` / `//go:build linux`) and have `_darwin` / `_linux` suffixes in filenames. Only macOS and Linux are supported; `internal/creds` does not build on other platforms.
- `test/e2e/e2e_test.go` is the cross-cutting smoke over a built binary.
- Test files are exempt from errcheck in `.golangci.yml`; production code must handle or explicitly join write/close errors.
