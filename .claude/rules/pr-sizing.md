# PR sizing

> Applies to: every PR. Cross-cutting workflow rule about when to split.

## The 800-line production-diff ceiling

A PR whose production diff exceeds 800 lines triggers a split decision. "Production diff" excludes `_test.go` files; test churn rides with its production code. Measure with:

    git diff --stat <base>..HEAD -- '*.go' ':!*_test.go'

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

## Why test diff is exempt

A migration that splits test rewrites away from their production code
produces a series of unreviewable PRs. The reviewer cannot tell whether
the production change is correct without the test that exercises it.
The 800-line rule is about cognitive cost in reviewing intent, and that
intent travels in `*.go` non-test files.
