# amuse

`amuse` controls Apple Music (Music.app) from the terminal on macOS, driven via AppleScript
(`osascript`). It's a personal project; keep additions minimal and avoid speculative features.

## Layout

- `main.go`, `cmd/` — cobra CLI entry point and subcommands
- `version/` — build-time version info (injected via `-ldflags` in `.goreleaser.yml`/`Makefile`)
- `internal/` — implementation details, not part of the public API

## Commands

- `make build` — build the `amuse` binary
- `make test` — run tests with coverage
- `make lint` — `golangci-lint run ./...` (global binary; version pinned in `ci.yml`, not `go.mod`)

## Tooling

- `govulncheck` is tracked as a `go.mod` `tool` dependency and invoked via `go tool govulncheck` so
  local dev and CI always resolve the same pinned version.
- `golangci-lint`, `gosec`, and `gitleaks` are intentionally **not** `go.mod` tools. golangci-lint
  was tried (see PR #5) but reverted a second time when it started blocking real runtime deps:
  golangci-lint v2's bundled `charm.land/lipgloss/v2` requires a newer `charmbracelet/x/ansi` than
  `charmbracelet/x/cellbuf` (a transitive dep of our own `bubbletea`/`lipgloss` v1 TUI stack)
  supports, and MVS has no compatible combination to pick — the build breaks module-wide, not just
  for the tool. gosec/gitleaks were reverted earlier for the same class of conflict (see PR #5).
  All three run as pinned GitHub Actions instead (`ci.yml`, `security.yml`). Don't move any of them
  back into `go.mod` without first confirming `go build ./...` still succeeds — this has broken
  twice now.
- Release automation: `tagpr` (`.tagpr`) opens the version-bump PR, merging it triggers
  `goreleaser` (`.goreleaser.yml`) to build/publish darwin binaries.
- `main` requires PRs (branch protection) — never push directly to `main`.

See [[.claude/rules/go-style.md]] and [[.claude/rules/testing.md]] for coding/testing conventions,
and `.claude/skills/ship.md` for this repo's branch → commit → PR workflow.
