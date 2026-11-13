# Git Workflow

## Where development happens
- Development happens in the workspace itself (the main checkout the user has open), on the feature branch. Do not create git worktrees, or other separate checkouts, for feature work unless the user explicitly asks.
- `master` is only used by the release pipeline. All development work happens on a feature branch until it is merged.
- Each feature or bugfix gets its own branch, named after its principal issue (see Branch naming), and linked to that issue in GitHub: create it with `gh issue develop <n> --name <branch> --base master --checkout`, and reference the issue in the PR (`Closes #<n>`).
- When a different feature branch is needed, switch the workspace to it, after checking the tree is clean (`git status --porcelain`). If there are uncommitted changes, ask the user.
- Give file paths relative to the workspace, so what the user sees matches what the agent works on.

## Branch naming
- Always name a feature or bugfix branch with GitHub's issue-branch convention: the issue number, then the issue title slugified (lower case, every run of non-alphanumerics becomes one `-`), e.g. `322-least-privilege-drop-group-member-readonly-scope`. This is the name GitHub's "Create a branch" (or `gh issue develop <n>`) gives for that issue.
- If the work has no issue, raise one first (or ask the user).

## CHANGELOG entries
- Feature and bugfix branches add their CHANGELOG lines under a `## Unreleased` section at the top of `CHANGELOG.md` (create it if missing), never under an already-tagged version.
- At release time the user renames `## Unreleased` to the version being tagged (for example `## v2.12.0`); only then do entries belong to a version heading.
- Keep one CHANGELOG line in the same commit as the change it describes, so a revert removes both.

## Versioning
- Releases use semantic versioning, `vX.Y.Z`:
  - **Z (patch):** a release that doesn't change behaviour (beyond bug fixes).
  - **Y (minor):** minor behaviour changes, such as features.
  - **X (major):** significant behaviour changes that require user action to upgrade safely.
- Each issue and PR names its target release as a GitHub milestone `vX.Y.Z` (create the milestone if it is missing).
- At release, `## Unreleased` in `CHANGELOG.md` is renamed to that version (see CHANGELOG entries).
- Release sequence, for each release:
  1. **Dry run:** the user triggers the release pipeline (`SSOSync-Build`) by hand on the master commit to be released, untagged. It isn't triggered automatically, to avoid multiple executions. It tests in all three staging accounts; the public release is skipped.
  2. **CHANGELOG section:** in a small docs-only PR (its own issue, on the release's milestone), rename `## Unreleased` in `CHANGELOG.md` to `## vX.Y.Z` and merge it. The release workflow's changelog-updater action then finds an existing section and doesn't generate a duplicate. Being docs-only, it needs no new dry run.
  3. **Tag:** the user tags the resulting master commit `vX.Y.Z`.
  4. **Release run:** the tag triggers the release pipeline. It publishes publicly, and the GitHub release workflow opens a chore PR ("Update CHANGELOG and Quick Start templates") that adds the release to the quick-start `#SemanticVersion` lists. Review its `CHANGELOG.md` diff (it should be empty or trivial), then merge it.
  5. **Clear:** once the release pipeline has completed the release run, the shared test environment is clear for developer-pipeline runs. Open feature branches are behind master by then (the CHANGELOG and chore-PR merges) and must be rebased (see "Before merging a PR") before their dev run.
- The dry run comes first so that, if it fails, fixes can be iterated on master (each in its own PR) until it passes. Only then is the release content final, and the CHANGELOG section is renamed to the version.
- Feature branches add a fresh `## Unreleased` section at the top of `CHANGELOG.md` if there isn't one.

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
