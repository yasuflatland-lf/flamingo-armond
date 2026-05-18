# Helpers introduced but not wired must be deleted

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.
> Application of the existing rule in
> [`.claude/rules/scope-discipline.md` § "Post-flight grep: helpers introduced but never wired"](../../../.claude/rules/scope-discipline.md#post-flight-grep-helpers-introduced-but-never-wired).

## Why

`User.UpdateProfile(DisplayName, Bio)` was introduced in Phase 1 of this PR
alongside the new domain VOs. The intent was to provide a single aggregate method
that encapsulated profile-update logic. However, both `usecase/user.go` and
`usecase/admin_user.go` retained the patch-based `repository.UserUpdate` flow and
never called `UpdateProfile`. The method had zero production callers at PR-completion
time.

A speculative API that ships with zero callers becomes dead exported surface. A
future contributor who finds `User.UpdateProfile` may:

- Route a new feature through it rather than the established `repository.UserUpdate`
  path, bypassing tests pinned to the per-file behavior.
- Assume the method is "the right way" without realizing no existing code uses it,
  leading to behavioral divergence.

The root cause was scope expansion: the VO introduction phase added a convenience
aggregator before the callers were ready to use it. The correct fix is to introduce
the aggregator in the same commit that wires the first caller.

## What

`User.UpdateProfile` was deleted in commit `234519d` in the same PR. The post-flight
check that caught this was:

```bash
grep -rn "UpdateProfile" backend/ --include='*.go' | grep -v _test.go | grep -v domain/user.go
```

Zero results confirmed the method had no production callers outside its own file.
The existing test in `domain/user_test.go` was also removed, leaving no dead test
coverage.

## The rule

After any route-through PR that introduces a domain or aggregate helper alongside
a refactor, run the post-flight grep:

```bash
grep -rn <NewSymbol> backend/ --include='*.go' | grep -v _test.go | grep -v <declaring-file>
```

A count of `0` means delete the helper in the same PR. "Keep for later" produces
the exact surface-rot described above.

## The rule applies per symbol, not per category — keyed on wired-ness

A single PR can introduce several helpers of the same category (e.g. a
boundary helper, a `Scan` method, a `Value` method, a `String` accessor) at
the same time. The rule applies to each helper *independently*: the
post-flight grep keys on wired-ness, not on what kind of method it is. The
current backend VOs illustrate the per-symbol verdict:

| Symbol | Caller count outside declaration | Verdict |
|---|---|---|
| `domain.BioFromPtr` | 1 (`repository/user.go:userToDomain`) + 1 test fixture | Keep — load-bearing read-path bridge. |
| Hypothetical `(Bio).Scan` / `(Bio).Value` (gorm row stores `*string`, never reaches the VO) | 0 | Delete in the same PR if introduced. |
| `(CardText).String` | 2 (`usecase/card.go` patch construction) | Keep — wired by the patch DTO boundary. |
| Hypothetical `(DisplayName).String` (no caller wired) | 0 | Delete in the same PR if introduced. |

A reviewer's "no tests" finding is not by itself a signal to delete. The
right next step is to grep for production callers:

- Zero callers → delete (the test gap is consistent with the call-site gap).
- One or more callers → keep, and either add the missing test or accept that
  the call-site behavioural tests cover the helper transitively.

The shorthand "untested means dead" elides the second case and recommends
deleting helpers that production code actually depends on. The reverse —
"wired means keep" — is also asymmetric: a helper with one production caller
and no direct unit test is fine if the caller's behavioural tests exercise
the helper's path; the helper is greppably reachable from a tested call site.

## Reference

- See [`.claude/rules/scope-discipline.md`](../../../.claude/rules/scope-discipline.md)
  for the general rule and the analogous example from issue #181 (`ResolvePageSize`).
- `backend/internal/domain/bio.go` — `BioFromPtr` (kept; wired by repository
  read path) is co-located with `Bio.Ptr()` / `Bio.IsSet()` (kept; wired by
  patch construction).
- `backend/internal/usecase/card.go` — `front.String()` / `back.String()` are
  the live callers that keep `CardText.String()` from being deleted.
