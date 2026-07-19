---
paths:
  - "**/*_test.go"
---

# Go testing conventions

- Use table-driven tests (`[]struct{ name string; ... }` + `t.Run(tt.name, ...)`) for anything with
  more than one case.
- Test names describe behavior, not implementation (`TestPlay_ReturnsErrorWhenMusicAppNotRunning`,
  not `TestPlay_Case1`).
- Code that shells out to `osascript` must be tested behind an interface/seam (inject a runner), not
  by actually invoking AppleScript/Music.app in tests — CI runs on macOS but there's no guarantee
  Music.app is in a testable state.
- Run `make test` (`go test ./... -coverprofile=coverage.out -covermode=count -count=1`) before
  considering work done; it must pass on both local and CI.
- Don't add tests for scenarios that can't happen; don't mock things that don't need mocking.
