---
name: ship
description: Branch, commit, push, and open a PR following this repo's conventions (branch protection, release.yml labels, tagpr labels).
---

Use this when the user asks to commit and open a PR for the current changes in this repo.

## Repo-specific facts

- `main` has branch protection: direct pushes are rejected, all changes need a PR.
- Squash-merge is the norm (`gh pr merge --squash`).
- Changelog categories come from PR **labels**, defined in `.github/release.yml`:
  `breaking change`/`breaking-change`, `enhancement`, `bug`, `dependencies` — anything else falls
  into "Other Changes". Apply the label that matches the change when creating the PR
  (`gh pr create --label ...` or `gh pr edit --add-label ...`).
- `tagpr` (`.tagpr`) decides the version bump from `tagpr:major` / `tagpr:minor` PR labels (default
  is patch). Only add one of these if the change is actually a major/minor-worthy API change —
  most PRs need neither.
- `.github/pull_request_template.md` defines the expected PR body shape (Summary + Test plan).

## Steps

1. `git status` — confirm what's changed. Never run destructive git commands without checking this
   first.
2. Sync with `origin/main`: `git fetch origin`, then fast-forward `main` locally
   (`git merge --ff-only origin/main`) if it's behind.
3. Create a branch from up-to-date `main` with a name describing the change
   (e.g. `fix/gitleaks-tool-conflict`).
4. Commit with a message that states what changed and why (not a restatement of the diff). Include
   the standard `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>` trailer.
5. Push and open the PR with `gh pr create --base main --head <branch>`, filling the Summary/Test
   plan sections, and applying the matching `release.yml` label (and `tagpr:major`/`tagpr:minor`
   only if warranted).
6. Watch checks: `gh pr checks <number> --watch --interval 15`.
7. Report pass/fail. Ask before merging — don't merge automatically unless the user has said to for
   this specific PR.
