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

- **Claimed edits may not be on disk.** When a sub-agent reports completing an implementation or test change, verify the change actually landed before proceeding: run `grep -n <expected-string> <file>` or `git diff --stat HEAD`. This is especially important when one agent was asked to make multiple distinct edits — verify each item individually. A sub-agent can self-report "done" after a tool-call failure, a wrong working directory, or an environment mismatch, leaving the file unchanged; the commit agent then silently omits it and the gap is only discovered downstream.

- **Review sub-agents regularly misread diff direction in refactor PRs.** When a review sub-agent claims that "tests were deleted from file X" or "component A handed responsibility to component B", read the diff yourself with `git log --oneline` and `git diff <base>..HEAD -- <file>` before acting. Refactors that move code between files are the highest-risk case: the agent may invert the direction of the move (reporting A→B when the actual change is B→A) or report deletions for lines that were in fact added. Treat any structural-change claim from a review sub-agent as unverified until you confirm it against the raw diff.

- **Review sub-agent findings may reverse a deliberate user scoping decision — ask, do not auto-apply.** A review agent does not see the conversation that scoped the change and will sometimes flag a deliberate choice (e.g. inlining N stubs into an L3 rule rather than keeping them as separate `docs/` files) as a regression. Before reverting in response to such a finding, summarize the conflict for the user and ask which direction to take — auto-applying the reviewer's fix silently undoes the prior decision. Worked example: a doc-cleanup pass deliberately inlined fifteen single-paragraph stubs into a rule file at the user's request; a follow-up code-reviewer pass flagged the inline as a tier-discipline regression and the conflict had to be surfaced rather than auto-resolved.

## Mechanical migrations: fan out per file, serialize the commit

A migration that applies the same substitution across N files (e.g. swapping every `gqlerr.*` return in `backend/internal/usecase/*.go` for typed `ucerr.*` errors) parallelises cleanly because the per-file diffs do not touch each other's source. The dispatch rules that make this safe:

1. **One subagent per file, with the file path baked into the prompt.** Two agents asked to "migrate the usecase files" without explicit file ownership will both edit `card.go`, producing a merge conflict or — worse — a partial overwrite that the second agent does not detect.
2. **No git operations inside the migration subagents.** Each migration agent only edits source. A single dedicated commit subagent runs serially after all migration agents report success, stages the produced diffs, and writes the commit. Parallel `git add` / `git commit` calls race for the `.git/index.lock` file and one will fail.
3. **The grep that scoped the migration may miss inline classifier calls.** A migration that targets only the most visible idiom (e.g. `assertGQLErr(t, err, ...)`) leaves callers of the lower-level classifier API (`gqlerr.IsCode(err, gqlerr.CodeInternal)`, `errors.AsType[*gqlerror.Error]`) untouched. Before dispatching, grep the package for every classifier shape the migration intends to remove, not just the most common one; otherwise the CI gate that forbids the old API still fails after the agents report done.

The post-conditions are the same as in [`docs/doc-organization.md` § "Parallel agent safety for doc splits"](../../docs/doc-organization.md#parallel-agent-safety-for-doc-splits) — single source ownership, no cross-agent edits — but the workflow is named differently (mechanical code migration vs. doc tree split) because the verification harness is `go build && go vet` rather than `markdown-link-check`.

### Scope `git add` to a known file list — never `git add <directory>` while siblings are mid-edit

The per-file fan-out above keeps source edits non-conflicting, but the commit
agent's `git add` step can still race the implementation agents if it stages by
directory. A `git add backend/internal/usecase/` issued while one implementation
agent is mid-edit of `swipe.go` will silently capture the partially-written
buffer; the commit lands with broken Go and the implementation agent's
subsequent `Write` is left as an unstaged delta the next commit attempts to
fix-forward. The race window is small but real on parallel toolchains.

Three mitigations, in increasing strictness:

1. **Stage by explicit file list.** The commit agent's prompt must enumerate
   the exact files to stage (`git add backend/internal/usecase/swipe.go backend/internal/usecase/swipe_test.go`),
   not the parent directory. The list comes from the implementation agents'
   completion reports, not from a `git status` snapshot taken inside the
   commit agent.
2. **Verify the working tree matches the expected list before staging.** Run
   `git diff --name-only HEAD` and assert the output is a superset of the
   expected file list. Any unexpected entry — typically a sibling agent's
   in-flight buffer — is a signal to wait, not to stage.
3. **Serialize commits behind implementation.** The strongest guarantee:
   no commit agent is dispatched until every implementation agent has
   reported done. The commit agent then runs alone with no concurrent
   writers, and the staging command's scope no longer matters.

The doc-split equivalent of this race — two agents editing the same anchor in
a shared sink file — is described in
[`docs/doc-organization.md` § "Parallel agent safety for doc splits"](../../docs/doc-organization.md#parallel-agent-safety-for-doc-splits)
and is the same shape of failure (concurrent writers to a single resource);
the mitigations there generalize.

## Pair reviewers with non-overlapping blind spots

A single review agent does not exhaust the failure modes of a change. `comment-analyzer` is keyed on identifier-level staleness (a function name in prose that no longer exists in code) while `code-reviewer` is keyed on structural and tier-discipline regressions (duplicate `##` headings, files in the wrong tier, missing cross-references). Either one alone misses what the other catches. For doc-cleanup or refactor passes that touch both prose accuracy and structural shape, run both review agents and merge their findings before acting. Worked example: a doc-cleanup pass surfaced a stale identifier reference only via `comment-analyzer` and a duplicate-heading regression only via `code-reviewer` — running just one would have shipped one of the two defects.
