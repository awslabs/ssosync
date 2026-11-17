# Delegation

- Once an action is agreed (watching a pipeline run, applying a fix, opening a PR, deploying), do not run it in the main conversation thread.
- Fork it to a workflow (`run_workflow`) and supervise from the main thread: relay questions, report outcomes, and keep the user's approval gates (push, force push, PRs) in the brief.
