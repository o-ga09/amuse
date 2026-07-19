---
paths:
  - "**/*.go"
---

# Go implementation conventions

Follow standard Go style: [Effective Go](https://go.dev/doc/effective_go) and the
[Google Go Style Guide](https://google.github.io/styleguide/go/). `.golangci.yml` in this repo is
the enforced source of truth — run `make lint` (`go tool golangci-lint`) before considering work done.

- Wrap errors with `%w` and add context; don't discard errors.
- Keep `internal/` packages internal; only `cmd/`, `main.go`, and `version/` are part of the public surface.
- Prefer small, focused functions over deep nesting; return early on error.
- No comments that restate what the code does — only comments that explain a non-obvious *why*.
- `gofmt`/`goimports` formatting is mandatory (enforced by lint).
