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
- `make lint` — `go tool golangci-lint run ./...`

## Tooling

- `golangci-lint` and `govulncheck` are tracked as `go.mod` `tool` dependencies and invoked via
  `go tool <name>` so local dev and CI always resolve the same pinned version. Do not install them
  globally and call the global binary instead.
- `gosec` and `gitleaks` are intentionally **not** `go.mod` tools — tracking either alongside
  golangci-lint pulled in unrelated/incompatible transitive dependencies (see git history on the
  `tools-go-mod` branch/PR #5 for what broke). They run as pinned GitHub Actions in
  `.github/workflows/security.yml` instead. Don't try to move them back into `go.mod` without
  re-verifying the module still builds cleanly.
- Release automation: `tagpr` (`.tagpr`) opens the version-bump PR, merging it triggers
  `goreleaser` (`.goreleaser.yml`) to build/publish darwin binaries.
- `main` requires PRs (branch protection) — never push directly to `main`.

See [[.claude/rules/go-style.md]] and [[.claude/rules/testing.md]] for coding/testing conventions,
and `.claude/skills/ship.md` for this repo's branch → commit → PR workflow.
