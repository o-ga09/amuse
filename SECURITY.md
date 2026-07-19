# Security Policy

## Supported Versions

Only the latest release is supported with security updates.

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please use [GitHub Private Vulnerability Reporting](https://github.com/o-ga09/amuse/security/advisories/new) to submit a report.

You should receive a response within 7 days. If the vulnerability is confirmed, a fix will be released as soon as possible.

## Scope

Reports related to the following are in scope:

- AppleScript/command injection via crafted input (e.g. playlist or track names) reaching `osascript`
- Unauthorized access to Music.app data or controls beyond what the invoking user already has
- Insecure handling of credentials/tokens (relevant if MusicKit API support is added in the future)
- Remote code execution

### Threat model

`amuse` is a local CLI tool that shells out to `osascript` to control Music.app on the same machine
it runs on. It does not listen on any network port and does not run with elevated privileges beyond
the invoking user's own session. The user already has OS-level control over Music.app equivalent to
what `amuse` can do on their behalf, so reports that assume a privilege boundary between the local
user and `amuse` itself are out of scope.

In-scope issues are ones where `amuse` does something the invoking user did not ask for — most
importantly, injection into the AppleScript commands it constructs from arguments (track/playlist
names, search terms) that could execute unintended AppleScript/shell behavior.
