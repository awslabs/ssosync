# Test Environment

## Google Workspace test tenant
- The pipelines (dev, release and the three staging accounts) share one dedicated Google Workspace test tenant; nothing else uses it.
- It is **billed per user**: be frugal, not frupid. To cover a new case, reuse or rename an existing test user (keeping the cases it already covers) rather than adding one. Propose a new user only when no existing user can carry the case, and say why.
- Each test user carries specific coverage; check `cicd/tests/attribute_map/google_test_users.json` and the run scripts before changing one (for example AceHole covers mononyms, #200; Olaf covers a suspended user).
- The Google Admin console **silently rejects adding a suspended user to a group** (no error). To add one: unsuspend, add to the group, re-suspend. A rename or OU move of a suspended user can drop their memberships the same way.
- After renaming or moving a test user, check every group membership the fixtures expect (`cicd/tests/attribute_map/baseline/groups.json`) in Google Admin, with the member filter showing suspended members, before starting a pipeline run.
- Tenant changes are made by the user in Google Admin, never by an agent. Every pipeline sees them at once, so a change that breaks old expectations must land in a window: branch rebased with the matching fixtures, no other pipeline running, tenant changed, dev run, then merge.

## Test files
- Don't create per-issue test files (e.g. `sync_issue<N>_test.go`). Add tests to the package's existing test file (e.g. `internal/sync_test.go`), with a comment above each test block naming the issue number and title.
