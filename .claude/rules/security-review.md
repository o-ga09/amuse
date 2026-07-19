---
paths:
  - "**/*.go"
---

# Security self-review

Before considering any change to this repo done, check it against these vulnerability classes.
Automated tools (`gosec`, `govulncheck`, `gitleaks` in `.github/workflows/security.yml`) catch some
of this, but review the logic yourself too — static analysis misses intent-level issues.

- **AppleScript/command injection (this repo's main attack surface).** Any code that builds an
  `osascript`/AppleScript string from a variable (track name, playlist name, search term, etc.) must
  escape or otherwise neutralize quotes and control characters before interpolating it, or pass the
  value in a way that can't be interpreted as AppleScript syntax. Never string-concatenate raw
  external input directly into a script passed to `exec.Command("osascript", "-e", ...)`.
- **Shell/argument injection.** Pass arguments to `os/exec` as separate slice elements, never build a
  single string and hand it to a shell.
- **Path traversal.** If a path is derived from user/external input, validate it stays within the
  intended directory before reading/writing.
- **Secrets.** Never log, print, or hardcode tokens/credentials. If MusicKit tokens are ever added,
  they belong in the OS keychain or an OS-permissioned file, not plaintext config committed to git.
- **Untrusted deserialization.** Prefer `encoding/json` over formats that can execute code; don't
  deserialize AppleScript/exec output as anything other than plain text/structured data.

If a change touches any of the above, say so explicitly when reporting the work as done, including
what was done to mitigate it.
