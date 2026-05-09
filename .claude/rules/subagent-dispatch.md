# Sub-agent dispatch

> Applies to: any session that orchestrates parallel sub-agents via the `Agent` tool, especially the 5-phase implement → review → simplify → test → document pipeline. Cross-cutting workflow rule.

## Tool access must match the work

When dispatching a sub-agent for implementation work — anything that should produce code on disk — pick a `subagent_type` that has `Edit` and `Write` access. Read-only or design-only agents (e.g. `feature-dev:code-architect`, `feature-dev:code-reviewer`, `Explore`) return blueprints and prose, not committed changes; if the parent agent expected files on disk, the next phase blocks because the prior phase produced nothing actionable.

| Phase | Use these subagent types |
|---|---|
| Implementation, refactor, simplify | `general-purpose`, `code-simplifier:code-simplifier`, `pr-review-toolkit:code-simplifier` (any agent with `Tools: All tools` or `Tools: *`) |
| Review, design, exploration | `feature-dev:code-architect`, `feature-dev:code-reviewer`, `feature-dev:code-explorer`, `Plan`, `Explore` |

The agent description's `(Tools: ...)` line is the source of truth — read it before dispatching, not the agent name.

## Verify findings against primary sources

Sub-agents have repeatedly:

- Reported "missing tests" for tests that exist (the agent didn't search the right path).
- Misinterpreted regenerated `gqlgen` / `codegen` diffs as bugs (the diff is mechanical, not semantic).
- Introduced convention-violating code (e.g. `eval`-based shell, relative-path Markdown links) that the parent has to revert.

Before acting on a sub-agent's finding — opening an issue, reverting a commit, escalating to the user — open the file or run the command yourself. The verification cost is cheap; the rework cost when the finding is wrong is not.
