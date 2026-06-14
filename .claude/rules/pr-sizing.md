# PR sizing

> Applies to: every PR. Cross-cutting workflow rule about when to split.

## The 800-line production-diff ceiling

A PR whose production diff exceeds 800 lines triggers a split decision. "Production diff" excludes `_test.go` files; test churn rides with its production code. Measure with:

    git diff --stat <base>..HEAD -- '*.go' ':!*_test.go'

**Measure against the merge-base, not a moved-ahead `origin/main`.** If upstream advances after you branch (common when working in a long-lived worktree while other PRs land on `main`), `git diff origin/main..HEAD` (two-dot) reports the *reverse* delta of the unrelated upstream commits — you will see other people's files as if you deleted them, and your own files will be missing. Use three-dot `git diff origin/main...HEAD` (diffs against the merge-base) or `git diff <branch-point>..HEAD`; the two-dot and three-dot forms agree only while `origin/main` is still your exact base. The three-dot form matches the diff GitHub shows on the PR, which is always computed against the merge-base.

## When to decide

The split decision happens at design time, not after the first review round.
A reviewer who is 400 lines into a 1500-line PR has already paid the cost
the rule exists to prevent.

## Worked example: bulk-migration sub-PR shape (#158)

Migrations that touch N independent sites split along site-count lines, not
file boundaries:

- sub-PR A: largest-N usecase files (e.g., top 3 = ~99 sites)
- sub-PR B: medium-N usecase files (e.g., next 3 = ~36 sites)
- sub-PR C: smallest-N usecase files + handler shims (e.g., last 3 = ~16 sites)
- sub-PR D: harness flip (warn → error) + rule node + frontend e2e parity

D lands last so the gate flips only after every site is converted.

## Re-verify call-site count before sizing

Before estimating migration cost in a plan, grep the production tree directly.
Issue-body estimates are written at a point in time and become stale as prior
work lands. The canonical check:

    grep -rnE '<pattern>' backend/ --include='*.go' | grep -v '_test.go'

This takes seconds and is the authoritative count. Do not trust an issue body's
stated N-site figure without running the grep yourself.

**Worked example (issue #160 / gqlerr decoupling).** The issue body for Option A
cited a "151-site migration" cost. A scope-discovery grep returned zero
production hits because issue #158 had already converted every site. The actual
remaining work was ~50 lines. Acting on the stale estimate would have added
~1000 unnecessary lines to the diff.

## Why test diff is exempt

A migration that splits test rewrites away from their production code
produces a series of unreviewable PRs. The reviewer cannot tell whether
the production change is correct without the test that exercises it.
The 800-line rule is about cognitive cost in reviewing intent, and that
intent travels in `*.go` non-test files.
