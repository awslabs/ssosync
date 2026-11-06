# Git Workflow

## Changing a branch that has already been merged
- PRs in this repo are merged with **merge + squash**, so the branch's own commits never appear on `master`; only the single squash commit does.
- Before committing anything on a branch whose PR is already merged, **rebase it onto the current `master` first** (`git fetch origin` then rebase onto `origin/master`).
  - If commits were already made on top of the merged branch, replay only the new ones: `git rebase --onto origin/master <last-merged-commit>`.
- Check the PR state (`gh pr view <n> -R awslabs/ssosync --json state,mergeCommit`) before committing to an existing branch, so the merged case is noticed.
- A rebased merged branch needs a new PR; pushing it over the old branch name requires a force push, which needs explicit user approval. Prefer pushing to a new branch.

## Before merging a PR: up to date with master
- Whenever a feature or bugfix branch falls behind `master` (check `git fetch origin && git merge-base --is-ancestor origin/master <branch>`), it must be:
  1. rebased onto the current `origin/master`;
  2. conflicts resolved, and the local build and tests run (`make test`, plus cfn-lint for changed templates);
  3. pushed (updating an open PR's branch after a rebase is a force push: ask the user first);
  4. run through the **developer pipeline** (`SSOSync`, pointed at the branch) until it passes.
- Only then can its PR be merged. A PR whose last green dev run predates the rebase is not ready.
