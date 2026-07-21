# Changelog

## [v0.0.3](https://github.com/o-ga09/amuse/compare/v0.0.2...v0.0.3) - 2026-07-21

### Other Changes
- [fix] プレイリスト内トラック再生後に次曲へ進まない不具合を修正 by @o-ga09 in https://github.com/o-ga09/amuse/pull/33

## [v0.0.2](https://github.com/o-ga09/amuse/compare/v0.0.1...v0.0.2) - 2026-07-20

### New Features 🎉
- feat: show album artwork in the now-playing TUI by @o-ga09 in https://github.com/o-ga09/amuse/pull/19
- feat: add benchmarks to verify Go-native performance by @o-ga09 in https://github.com/o-ga09/amuse/pull/26
- docs: document the &quot;computer not authorized&quot; artwork error (#21) by @o-ga09 in https://github.com/o-ga09/amuse/pull/27
- feat: ローカルライブラリ検索/一覧コマンド (search/songs) を追加 (#17) by @o-ga09 in https://github.com/o-ga09/amuse/pull/29
- feat: add arrow-key navigable playlist/track browser to the TUI by @o-ga09 in https://github.com/o-ga09/amuse/pull/30
- feat: add playlist create/delete/add/remove management to the TUI by @o-ga09 in https://github.com/o-ga09/amuse/pull/31
### Fix bug 🐛
- feat: auto-refresh the now-playing TUI on a timer by @o-ga09 in https://github.com/o-ga09/amuse/pull/24
- [fix] キュー末尾で next/prev がプレイリスト先頭に戻るようにする by @o-ga09 in https://github.com/o-ga09/amuse/pull/25
### Other Changes
- Add cmd/ tests, Homebrew install CI, and octocov coverage reporting by @o-ga09 in https://github.com/o-ga09/amuse/pull/32

## [v0.0.1](https://github.com/o-ga09/amuse/commits/v0.0.1) - 2026-07-19

### New Features 🎉
- chore: add project-level Claude Code config, issue/PR templates by @o-ga09 in https://github.com/o-ga09/amuse/pull/6
- feat: add play/pause/next/prev/now commands by @o-ga09 in https://github.com/o-ga09/amuse/pull/7
- feat: add interactive TUI, revert golangci-lint go.mod tracking by @o-ga09 in https://github.com/o-ga09/amuse/pull/8
- feat: add shuffle/repeat/volume, CLI and TUI by @o-ga09 in https://github.com/o-ga09/amuse/pull/9
- feat: add a startup banner (TUI + README) by @o-ga09 in https://github.com/o-ga09/amuse/pull/10
- feat: publish a Homebrew formula on release by @o-ga09 in https://github.com/o-ga09/amuse/pull/12
### Fix bug 🐛
- fix: render the README banner as vector rects, not font-dependent text by @o-ga09 in https://github.com/o-ga09/amuse/pull/11
### Dependency Updates ⬆️
- chore(deps): Bump the dependencies group across 1 directory with 2 updates by @dependabot[bot] in https://github.com/o-ga09/amuse/pull/1
### Other Changes
- ci: split into lint/test/govulncheck jobs by @o-ga09 in https://github.com/o-ga09/amuse/pull/2
- security: add SAST/gitleaks, Dependabot auto-merge, bump go directive by @o-ga09 in https://github.com/o-ga09/amuse/pull/4
- tools: manage lint/security tools via go.mod, English CI text by @o-ga09 in https://github.com/o-ga09/amuse/pull/5
