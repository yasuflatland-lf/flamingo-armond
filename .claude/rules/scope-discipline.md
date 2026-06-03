# Scope discipline

> Applies to: every request. Cross-cutting workflow rule about what *not* to do before answering.

## Don't grep the codebase to answer external questions

Questions about external platforms — GCP IAM, GitHub Environments semantics, Vercel build hooks, Supabase auth internals, npm tarball layout — cannot be answered by reading this repo. The repo only knows how *we* configure the platform; it does not know what the platform does. Examples that wasted time in past sessions:

- "Does GCP progressively remove `setIamPolicy` from `roles/editor`?" → answered by GCP IAM release notes, not by `iam.tf`.
- "What does GitHub Environments require for protection rules?" → answered by GitHub docs, not by `.github/workflows/*.yml`.

For external questions, fetch the official docs (`WebFetch`) or run a probing CLI (`gcloud … describe`, `gh api …`) and cite the source. If you can't verify, say `unverified` explicitly rather than reasoning from memory.

## Don't split files unless they exceed the documented threshold

When asked to apply a doc-tier or file-size convention (see `docs/doc-organization.md` and the L1/L2/L3 tier rules in `CLAUDE.md`), only split files that **actually exceed** the documented soft cap. Splitting files preemptively because "they're getting close" creates churn, breaks anchor links across the repo, and inverts the cost/benefit of the convention. Measure first (`wc -l`), split second.

## Verify claims about external platforms before stating them as fact

Statements about GCP, Vercel, Supabase, GitHub, or any other vendor's behavior must be backed by a primary source on the same edit:

- Vendor docs page (`WebFetch` the URL).
- Vendor CLI output (`gcloud`, `vercel`, `supabase`, `gh api`).
- Repo-local config that *is* the source of truth (e.g. `iam.tf` for our GCP IAM, but only for our config — never for vendor behavior).

A confident-sounding wrong answer about platform behavior costs more downstream than a hedged "I haven't verified this" answer up front.

The same principle applies to **library behavior disputes** between agents or between agent and reviewer. When two
agents disagree on what a library does (e.g. whether `usePathname()` returns a decoded or percent-encoded string),
resolve the disagreement by consulting a primary source — read the installed package source, run the code, or fetch
the official changelog — on the same edit. Do not settle the dispute by majority vote or by deferring to whichever
side sounds more confident. Two concrete cases from this codebase:

- `usePathname` encoded vs decoded — settled by reading installed `next@16.2.4` source: `new URL(canonicalUrl, ...).pathname` preserves reserved characters as `%XX`. The "decoded" reading of the docs was misleading; the primary source was the installed code, not a summary.
- `pointer-events-none` vestigial vs load-bearing — settled by reasoning about `sticky`'s overflow-overlay semantics: the dead-zone cost is invisible (no interactive element sits under the safe-area padding zone), while removing `pointer-events-none` blocks scroll gestures near the bar in overflow viewports. The primary source was the CSS spec behavior, confirmed by the layout constraints.

## Plan-document claims about the codebase's structure must be toolchain-verified

When a plan asserts a structural property of *our own* code — "no import cycle", "no transitive dependency on package X", "this package compiles standalone", "this generic type erases to a single instantiation" — the proof must come from the toolchain (`go build`, `go list`, `go vet`, `tsc --noEmit`), not from a manual reading of the import statements or type signatures listed in the plan. The plan author is reasoning about a snapshot of intent; the toolchain reasons about the actual current source tree, including legacy state the plan summary may have abstracted away.

A worked example: a plan asserted that adding a `gqlerr → usecase` import would not create a cycle, on the basis that the new shared types in `usecase` had no `gqlerr` import. The legacy `usecase/*.go` production files already imported `gqlerr` (a fact the plan summary did not surface), and the proposed direct import would have closed a cycle the compiler rejects. A 5-second smoke build (`echo 'package main; import _ "backend/internal/gqlerr"; func main() {}' | go build -`, scoped to the proposed layout) catches this before any other infrastructure is wired. Run the toolchain check before committing to a directory layout; do not settle the question by re-reading the plan.

A second worked example: before writing a `go-arch-lint` archfile that encodes 21 component dependency rules, run `go list -f '{{join .Imports "\n"}}' ./internal/usecase/...` (and analogous invocations for every component) to enumerate the actual imports. Any mismatch between the plan's stated `mayDependOn` list and the real import set produces either a violation at first run (too restrictive) or a silent gap in enforcement (too permissive). The pre-flight `go list` survey takes under a minute for the whole tree and eliminates an entire class of "rule doesn't match reality" iteration rounds.

## Pre-existing inconsistency surfaced by an adjacent edit

When a PR's diff converts adjacent lines in a file to a new style (e.g. bare-text `§` reference → Markdown anchor link), other pre-existing bare-text references in the **same file** may become visibly inconsistent with the just-edited lines. The bug existed before the PR — `git show main:<file>` confirms it — but the *visibility* and the in-file style asymmetry were introduced by the current change.

Default posture: **the same PR fixes the adjacent (visually contiguous) pre-existing instances** when:

- They live in the same file as the lines just edited.
- They follow the same broken pattern (e.g. the same `docs/foo.md §` failure mode).
- The fix cost is bounded (one-line per site, no semantic change).

A worked example from this repository: a change converted `docs/deployment.md:106, 297` from bare-text `docs/backend.md § "X"` references to anchor links. Two siblings in the same file (`L108`, `L301`) carried the identical broken pattern as pre-existing bugs. They were brought into the PR scope because the in-file style was now mixed; the diff stayed bounded and the fixes were one-line each. The further-away `L135, L303` and 13+ similar sites in other files stayed out of scope — those did not become inconsistent with anything the PR had touched.

The boundary: same file, same pattern, same edit-shape. Going beyond is the cleanup-by-the-side-of-the-road that the broader rule above forbids.

The same principle applies to **test files**. When a PR introduces a new test file for a function (rather than appending to an existing one), pre-existing untested branches of that function become the new file's responsibility — even if the PR's stated intent only covered one new branch. The new file is now the canonical location for testing the function; leaving other branches uncovered sends the signal that the file is complete when it is not. A concrete example: a new `ownership_test.go` added tests for the context-pass-through branch of `authorizeCardgroupOrBadInput` and `authorizeCardgroupOrUnauthenticated`; a reviewer found that the success path and the generic infrastructure-error wrap path were also untested. Because the new file was the only test coverage for those helpers, those branches became in-scope for the same PR. See [`docs/backend/library-gotchas/direct-unit-test-for-shared-helper.md`](../../docs/backend/library-gotchas/direct-unit-test-for-shared-helper.md) for the branch-map technique.

The same principle applies to **env-var and constant deletions**. When a production constant or env-var read is removed from the codebase, any place that documents or sets that variable becomes inconsistent: `backend/.env.example`, `docs/backend.md` (table rows or prose), and infra configs (`render.yaml`, `cloudbuild.yaml`, terraform). Those references should be cleaned in the same PR as the code deletion — the inconsistency is surfaced by the adjacent code change, so the scope boundary is the same as for style-fix and test-file cases above.

A worked example: deleting `cfg.swipeNextBatchSize` from `backend/cmd/server/main.go` left `SWIPE_NEXT_BATCH_SIZE` referenced in `backend/.env.example` (line 4) and `docs/backend.md` (table row and prose mention). Both were removed in the same PR via a separate `chore(env)` commit. The grep that discovers the stragglers is:

```bash
grep -rn 'SWIPE_NEXT_BATCH_SIZE' backend/ docs/ render.yaml cloudbuild.yaml
```

The boundary: infra configs (`render.yaml`, `cloudbuild.yaml`, terraform) may require operator sign-off if removing the variable from deployed services. Surface those separately and ask — do not auto-remove without confirmation.

### Code-path deletion obliges test deletion in the same PR

When a production code path is removed (not just renamed or moved), the tests that exercised it become dead. Dead tests must be deleted in full, not gutted — leaving them as no-ops or orphan assertion shells pollutes the suite with assertions that prove nothing and silently signal "this path is still covered" when it is not.

Gutting a test is the harder-to-detect failure mode: the test file compiles, CI is green, and a future reader cannot tell at a glance whether the assertion still exercises real code. A fully deleted test is unambiguous — it is simply gone.

Worked example: `backend/internal/usecase/swipe.go` lost its `FindDueCardsForUserTx` call and its `ordering.Apply` step. The two tests that targeted those paths — `TestSwipeUsecase_HandleSwipe_FindDueCardsError_PinsChain` and `TestSwipeUsecase_HandleSwipe_FindDueCards_PropagatesCancelled` — were deleted in full, not converted to no-ops. A test for the dropped env-var read (`TestServerConfig_SwipeNextBatchSize`) was also deleted because the env-var read no longer exists in `backend/cmd/server/main.go`.

The boundary: this rule applies only when the **code path** is gone. If the path is renamed or refactored, update the test to match the new shape. Cross-reference: see [`docs/backend/error-wrapping/redundant-tests-after-alias-bridge-deletion.md`](../../docs/backend/error-wrapping/redundant-tests-after-alias-bridge-deletion.md) for the symbol-removal variant.

## Re-verify call-site count before sizing

Before estimating migration cost in a plan, grep the production tree directly. Issue-body estimates are written at a point in time and become stale as prior work lands. The canonical check:

    grep -rnE '<pattern>' backend/ --include='*.go' | grep -v '_test.go'

This takes seconds and is the authoritative count. Do not trust an issue body's stated N-site figure without running the grep yourself.

**Worked example (issue #160 / gqlerr decoupling).** The issue body for Option A cited a "151-site migration" cost. A scope-discovery grep returned zero production hits because issue #158 had already converted every site. The actual remaining work was ~50 lines. Acting on the stale estimate would have added ~1000 unnecessary lines to the diff.

### Post-flight grep: helpers introduced but never wired

The pre-flight grep above measures incoming scope; the **post-flight grep** measures outgoing scope. After a route-through PR (introduce a shared helper, then convert all callers to use it), grep for the helper's caller count and confirm it matches the planned count. A zero-caller result is the signal that the helper was added speculatively, never wired, and should be removed in the same PR.

A worked example from issue #181: `ResolvePageSize` was introduced in Phase 1 alongside `TrimAndDetect`, anticipating that `admin_user.go` / `card.go` / `cardgroup.go` would converge on a single page-size resolver. Phase 2's route-through audit showed each file kept its own variant (`resolveAdminPageSize`, etc.) because their behavioral nuances did not collapse cleanly. The `ResolvePageSize` declaration thus had zero callers at PR-completion time and was deleted in the same PR (commit 3e0f9c8). A speculative helper left behind becomes dead exported API that a future contributor will find before noticing the per-file variants, potentially merging code through a path that bypasses tests pinned to the per-file behavior.

The grep contract is:

```bash
# After route-through, every shared helper introduced in the PR
grep -rn <NewHelperName> backend/ --include='*.go' | grep -v _test.go | grep -v <helper-file>
```

A count of `0` means delete the helper. A count below the planned target means investigate why route-through stopped short. A count matching the plan means the helper is wired correctly.

## Constructor-signature migration: include resolver-layer test files in the pre-flight grep

When a migration changes a usecase constructor signature (e.g. replacing a raw `AdminChecker` interface with a `*AdminGate` wrapper), the pre-flight grep must include `backend/graph/resolver/` in addition to `backend/internal/usecase/`. Resolver test files (`backend/graph/resolver/*_test.go`) instantiate usecase constructors directly to build integration-level harnesses; they are call sites in exactly the same sense as `*_test.go` files under `internal/usecase/`.

A grep scoped only to `backend/internal/usecase/` will show N sites and miss any resolver test that constructs the usecase under test. The migration lands, tests in `internal/usecase/` pass, and then the resolver test fails to compile.

The authoritative pre-flight grep for a constructor change:

```bash
grep -rnE 'New<UsecaseName>\(' backend/ --include='*.go'
```

This covers `internal/usecase/`, `cmd/server/`, and `graph/resolver/` in one pass. Any file that calls the constructor — production or test — must be updated.

**Worked example.** A constructor migration grepped only `backend/internal/usecase/*.go` and listed the files to update. `backend/graph/resolver/card_import_resolver_test.go` was not in scope. The test constructed `NewCardImportUsecaseWithTx(...)` directly and failed to compile after the signature changed. The fix was small, but the gap required a follow-up commit. A full-tree grep before writing the migration plan would have enumerated every call site and the resolver file would have been in scope from the start.

### Adding a method to a repository interface fans out to every implementer, including test fakes

The same pre-flight discipline applies to **adding a method to an interface** — the interface-method-addition sibling of the constructor-signature case. When a method is added to a repository interface (e.g. `FindByIDsTx` on `repository.RoleRepository`), every type that implements the interface must gain the method or it stops satisfying it. The production repo is the obvious implementer; the easy-to-miss ones are the **test fakes and stubs** scattered across `internal/loader`, `cmd/server`, and `graph/resolver/*_test.go` that implement the same interface to build harnesses. A grep scoped to the production repo file misses all of them.

The authoritative discovery is the compiler — `go build ./...` names every type that no longer satisfies the interface. A grep complement enumerates the fakes directly:

```bash
grep -rn 'repository.RoleRepository' backend/ --include='*.go'   # production + every fake
go build ./...                                                    # names each non-implementer
```

Enumerate all implementers before writing the change, not after the first build failure: a fake hidden in a resolver test is in scope for the same change that adds the method.

### Frontend page-behavior changes: pre-flight grep BOTH frontend test trees

The frontend sibling of the constructor-signature rule above. Frontend tests live in two trees, and a page-behavior change is exercised from either or both:

- **Co-located narrow tests** under `frontend/src/app/**/*.test.tsx` (next to the source).
- **Broad page tests** under `frontend/__tests__/*.test.tsx` (a separate top-level tree).

The narrow/broad split is a contract — see [`docs/frontend/testing-convention-narrow-vs-broad-page-tests.md`](../../docs/frontend/testing-convention-narrow-vs-broad-page-tests.md). When changing a page's behavior, the pre-flight test-discovery grep MUST cover both trees, keyed on the route segment, not just the co-located file:

```bash
grep -rln "admin/users" frontend/src frontend/__tests__
```

A grep scoped to the co-located file alone (or an agent told to "update the test" that creates a fresh co-located test and stops) leaves any broad page test under `frontend/__tests__/` still pinned to the old behavior. That broad test breaks the full suite, and the gap surfaces only at the orchestrator's full-suite re-run — exactly the resolver-test-compile-failure shape of the backend rule, one tree over.
