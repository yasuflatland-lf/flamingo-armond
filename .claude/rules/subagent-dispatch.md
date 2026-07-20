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

- **In a git worktree, subagents must receive absolute worktree paths — never relative or basename-resolved paths.** The same tracked file exists in both the worktree (`<repo>/.claude/worktrees/<branch>/path/to/file.tsx`) and the primary checkout (`<repo>/path/to/file.tsx`). A subagent given a relative path, or one that searches by basename, can resolve to and edit the main-repo copy, then self-report "done" — leaving the worktree file stale and silently dirtying the main checkout. Mitigations: (a) pass the ABSOLUTE worktree path in every subagent prompt and name the main-repo path explicitly as "do not touch"; (b) require the agent to confirm with a post-edit grep that the worktree path changed AND the main-repo path did not; (c) the orchestrator runs `git -C <main-repo-root> status` after all worktree subagents finish and reverts any stray main-repo edit before staging.

- **Re-run lint, typecheck, and the test suite in the orchestrator — never trust a subagent's self-reported status.** Subagents have reported "lint passes" and a specific test count when the orchestrator's own run produced a lint failure and a different test count. Always re-run `lint` / `typecheck` / the full test suite in the orchestrator after agents claim green; treat agent tool-status claims as unverified until the orchestrator confirms them. Companion gotcha: `biome format --write` does NOT organize imports — only `biome check --write` applies the `assist/source/organizeImports` safe fix. An agent that runs `format` and reports "imports clean" has not run the import-sort step.

- **A sub-agent may make unauthorized edits despite explicit "do not modify" instructions.** A verification sub-agent told to "report violations, do not fix" can still produce file edits (config changes, placeholder files) if it determines the change is necessary to make the check pass. Always run `git diff` after any sub-agent completes — including read-only or verification agents — before proceeding to the next step. Unauthorized changes must be reverted before the commit agent stages files, otherwise they enter the history without review.

- **Commit verified work before dispatching any follow-up agent into the same working tree.** A confused agent can run a destructive git command even when instructed not to. A working-tree restore (`git checkout` / `git restore`-class) silently discards uncommitted edits and does NOT appear in `git reflog` (reflog only tracks HEAD moves), so the loss is unrecoverable when the work was never committed or stashed. The reliable guarantee is not the "no git" instruction in the follow-up agent's prompt (defense-in-depth at best) but that the work is already in a commit. In a fan-out → commit pipeline, dispatch the commit agent the moment implementation agents report done and their edits are verified on disk — before any simplify/format/lint-fix/review agent touches the tree. Treat a formatting or lint autofix as a separate commit on top of committed work, not a step run against a dirty tree.

- **Review sub-agents regularly misread diff direction in refactor PRs.** When a review sub-agent claims that "tests were deleted from file X" or "component A handed responsibility to component B", read the diff yourself with `git log --oneline` and `git diff <base>..HEAD -- <file>` before acting. Refactors that move code between files are the highest-risk case: the agent may invert the direction of the move (reporting A→B when the actual change is B→A) or report deletions for lines that were in fact added. Treat any structural-change claim from a review sub-agent as unverified until you confirm it against the raw diff.

- **Review sub-agent findings may reverse a deliberate user scoping decision — ask, do not auto-apply.** A review agent does not see the conversation that scoped the change and will sometimes flag a deliberate choice (e.g. inlining N stubs into an L3 rule rather than keeping them as separate `docs/` files) as a regression. Before reverting in response to such a finding, summarize the conflict for the user and ask which direction to take — auto-applying the reviewer's fix silently undoes the prior decision. Worked example: a doc-cleanup pass deliberately inlined fifteen single-paragraph stubs into a rule file at the user's request; a follow-up code-reviewer pass flagged the inline as a tier-discipline regression and the conflict had to be surfaced rather than auto-resolved.

## Mechanical migrations: fan out per file, serialize the commit

A migration that applies the same substitution across N files (e.g. swapping every `gqlerr.*` return in `backend/internal/usecase/*.go` for typed `ucerr.*` errors) parallelises cleanly because the per-file diffs do not touch each other's source. The dispatch rules that make this safe:

1. **One subagent per file, with the file path baked into the prompt.** Two agents asked to "migrate the usecase files" without explicit file ownership will both edit `card.go`, producing a merge conflict or — worse — a partial overwrite that the second agent does not detect.
2. **No git operations inside the migration subagents.** Each migration agent only edits source. A single dedicated commit subagent runs serially after all migration agents report success, stages the produced diffs, and writes the commit. Parallel `git add` / `git commit` calls race for the `.git/index.lock` file and one will fail.
3. **The grep that scoped the migration may miss inline classifier calls.** A migration that targets only the most visible idiom (e.g. `assertGQLErr(t, err, ...)`) leaves callers of the lower-level classifier API (`gqlerrtest.IsCode(err, gqlerr.CodeInternal)`, `errors.AsType[*gqlerror.Error]`) untouched. Before dispatching, grep the package for every classifier shape the migration intends to remove, not just the most common one; otherwise the CI gate that forbids the old API still fails after the agents report done.

The post-conditions are the same as in [`docs/doc-organization.md` § "Parallel agent safety for doc splits"](../../docs/doc-organization.md#parallel-agent-safety-for-doc-splits) — single source ownership, no cross-agent edits — but the workflow is named differently (mechanical code migration vs. doc tree split) because the verification harness is `go build && go vet` rather than `markdown-link-check`.

### When per-concern split forces same-file overlap, consolidate into one agent

The natural split for a multi-aspect migration is by concern: one agent for production code changes, one for assertion updates, one for test fixture rewrites. When a constructor-signature change affects many files in the same package, this per-concern split forces two agents to touch the same file — for example, an "assertion-stripping" agent and a "constructor-call-stripping" agent both editing `swipe_performance_test.go`. The conflict-avoidance rule from this section then demands a different split shape.

The fix is to collapse the per-concern agents into a single per-package "mega" agent that owns all edits to its files. This is slower (the agent runs serially across N files) but eliminates the race entirely. Concerns that do not overlap with the mega agent's file set can still run in parallel alongside it.

Worked example from issue #229: a 22-call-site `nextBatchSize int` parameter drop affected 7 files (`swipe.go`, `main.go`, `main_test.go`, `swipe_resolver_test.go`, `swipe_error_test.go`, `swipe_performance_test.go`, `card_test.go`). The original plan split work into 5 parallel agents by concern (1 for production code, 1 for resolver tests, 3 for usecase test sub-concerns). The conflict: an assertion-stripping agent and a constructor-call-stripping agent both targeted `swipe_performance_test.go` and `swipe_resolver_test.go`. The fix consolidated all 5 agents into a single "T2a-mega" agent owning all 7 files, completing serially in one pass with no conflicts. The remaining backend work (`mapper.go`) and frontend work (3 files) ran in parallel because they had no file overlap with T2a-mega.

**Decision criterion**: if the per-concern split would assign two agents to the same file, choose one of: (a) consolidate the concerns into one agent for that file's package — the default; (b) serialize the concerns with explicit dependency ordering — slower, but use this when one concern is qualitatively riskier and benefits from a dedicated reviewer before the next concern runs. Default to (a) for mechanical parameter-drop or signature-change migrations where all edits are similarly low-risk.

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

### Partial staging across waves: hand-crafted patches over `git add -p`

When a multi-Item PR assigns per-Item commits for reviewability but two or more Items share a file, a naive `git add <file>` collapses all of that file's hunks into one commit and destroys the per-Item story. `git add -p` interactive prompts are unreliable for automated agents — subagent tooling cannot answer "y/n/s" deterministically across hunk boundaries.

The mitigation pattern:

1. **Implementation agents complete all edits without staging.** No git operations inside implementation agents (see ["Scope `git add` to a known file list"](#scope-git-add-to-a-known-file-list----never-git-add-directory-while-siblings-are-mid-edit) above).
2. **A dedicated commit agent per wave splits shared files by hunk:**
   - Produce a per-Item patch via `git diff <file>` filtered to the target line ranges.
   - Apply it to the index via `git apply --cached --recount <patch>`. The `--recount` flag is required because hand-crafted patches violate git's strict hunk-line-count validator before content evaluation.
   - After that commit lands, the remaining unstaged hunks belong to the next commit — `git add <file>` (or `git add -A` for unshared files) picks them up.
3. **Verify each commit** with `git diff <commit>~..<commit> -- <file>` to confirm only the intended hunks landed.

Worked example: PR #204 Wave 1 split `usecase/card.go` between two Items (`BelongsToCardgroup` swap and `translateCardErr` removal); Wave 2 split `usecase/cardgroup.go` between two other Items (`ParseCardgroupName` swap and `uuidV7` → `domain.NewID`). Both splits used hand-crafted patches with `--recount`.

### Symbol moves must be atomic — one agent owns both delete and add

When moving a symbol (function, type, interface, constant) from one file to another **within the same Go package**, the deletion and the addition MUST happen in the same agent's edit batch. Splitting the two across parallel agents creates a duplicate-declaration build break in the intermediate state.

Worked example from issue #181 Phase 1:

- `H2`: created `admin_gate.go` (with `AdminChecker` interface) AND deleted the original `AdminChecker` declaration from `dictionary.go`. One agent owns both edits.
- `H5`: created `tx.go` (with `txRunner` type) AND deleted the original from `card.go`.

Parallel agents can still run, but each handles a **distinct symbol move**. Two agents touching the same symbol — one adding, one deleting — would race on the build state. If a symbol move is the *only* edit in a phase, a single sequential agent is sufficient; parallelism pays off only when several independent symbol moves share the phase.

### Compiler-driven fan-out has an unknowable file set — don't pair it with same-tree siblings

The per-file safety guarantee above assumes the migration's file set is known before dispatch. A compiler-driven migration — where the exact files to edit are enumerated by `go build ./...` naming each failing cast or type site — breaks that assumption. The fan-out set is revealed only as the agent works, not at dispatch time. Any file the compiler names is fair game for the migration agent, including files a sibling parallel agent was assigned.

Mitigation:

- **Treat a compiler-driven agent as owning the entire tree for its migration.** Do not assign any file that could plausibly appear in the compiler's error output to a sibling agent running in parallel. If the compiler could name it, the migration agent may edit it.
- **When the fan-out set is unknowable, choose one of:** (a) serialize the compiler-driven agent before or after all sibling agents; or (b) scope sibling agents strictly to files provably outside the fan-out — brand-new files, or a package the migration does not reference at all (verify via `go list`).
- **Verify post-hoc that both agents' changes landed in any shared file.** A lost concurrent write is silent: the file compiles, the agent self-reports done, and the missing edit is discovered only when the behaviour it was supposed to fix still appears. After all agents complete, grep for each expected edit in every file the sibling agents shared.

Worked example (issue #438): a typed-ID migration agent (compiler-driven cast fan-out for `domain.UserID` / `domain.CardgroupID`) and a separate "review-nits" agent ran in parallel. The migration agent needed to cast a `HandleSwipeInput` literal inside `backend/cmd/server/main_test.go`; the nits agent was simultaneously adding a new test (`TestBuildResolver_NotionEnabledRetryConfigError`) to the **same file**. Both edits happened to survive, but it was a genuine same-file concurrent-write race — had the writes interleaved differently, one edit would have silently overwritten the other. A single pre-dispatch `go build ./...` dry-run would have revealed that `main_test.go` was in the compiler's error list, placing it off-limits for the sibling agent.

The ["When per-concern split forces same-file overlap, consolidate into one agent"](#when-per-concern-split-forces-same-file-overlap-consolidate-into-one-agent) subsection above handles the case where the file set IS known but two concern-based splits collide — use that rule when you can enumerate the overlap up front, and this rule when the overlap is only discoverable at compile time.

## Pair reviewers with non-overlapping blind spots

A single review agent does not exhaust the failure modes of a change. `comment-analyzer` is keyed on identifier-level staleness (a function name in prose that no longer exists in code) while `code-reviewer` is keyed on structural and tier-discipline regressions (duplicate `##` headings, files in the wrong tier, missing cross-references). Either one alone misses what the other catches. For doc-cleanup or refactor passes that touch both prose accuracy and structural shape, run both review agents and merge their findings before acting. Worked example: a doc-cleanup pass surfaced a stale identifier reference only via `comment-analyzer` and a duplicate-heading regression only via `code-reviewer` — running just one would have shipped one of the two defects.
