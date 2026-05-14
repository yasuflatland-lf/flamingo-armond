# Sentinel layering: when to join with `errors.Join` and when to keep standalone

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

When introducing a more specific sentinel alongside an existing general one, the question is whether the new sentinel **refines** the general case (in which case `errors.Join` keeps every existing caller working) or **inverts** it (in which case joining would lie to any caller branching on the general sentinel). The two sections below cover both halves of the rule.

## When to join with `errors.Join`

When introducing a more specific sentinel alongside an existing general one (e.g. adding `ErrUserNotFound` while `ErrNotFound` is still in use across the repository), return `errors.Join(specific, general)` from the new call site. Callers that already match `errors.Is(err, ErrNotFound)` keep working; callers that want the finer split can branch on the specific sentinel first. This avoids a flag-day rename across every caller and lets the finer sentinel migrate in at its own pace. **Always check the more specific sentinel before the general one** — `errors.Is` returns true for both, so reversing the order silently routes user-not-found into a generic 404 path.

## When NOT to join (standalone sentinels)

The layered-sentinel pattern (`errors.Join(specific, general)`) only works when the specific case is **semantically a refinement** of the general case. `ErrUserNotFound` and `ErrRoleNotFound` refine `ErrNotFound`, so joining is correct: a caller branching only on `ErrNotFound` still gets the right behaviour. But `ErrRoleDuplicate` is the inverse condition — the row was *found* and that is precisely the failure. Joining it with `ErrNotFound` would make `errors.Is(err, ErrNotFound)` true for a duplicate insert, which is a lie that any general-purpose 404 mapper would happily act on. Keep "found" sentinels (duplicate, conflict, already-exists) standalone; only "missing" sentinels get the `errors.Join` treatment.
