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

See [`.claude/rules/scope-discipline.md`](../../../.claude/rules/scope-discipline.md)
for the general rule and the analogous example from issue #181 (`ResolvePageSize`).
