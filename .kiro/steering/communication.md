# Communication

## Referring to GitHub items
- Issues: `#123` (issue number 123).
- Pull requests: `PR/437` (pull request number 437), never a bare `#437`.
- Applies to messages to the user, workflow briefs, reports and task notes. In text GitHub renders (PR bodies, issue comments, commit messages), GitHub's own syntax still applies (`Closes #123`, `#437` links either kind), but name pull requests as `PR/437` in prose where it is unambiguous to do so.

## Gates and ownership
- When something waits on the user's approval, label it **GATE** and make it the clear, separate ask.
- Keep actions separate (a push, a PR, a merge, a stack update are distinct items); merging is always the user's separate task.
- Label each item with who owns it (the user or the agent).
