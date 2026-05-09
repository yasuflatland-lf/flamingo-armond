# Standalone sentinels: not every new sentinel joins `ErrNotFound`

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

The layered-sentinel pattern (`errors.Join(specific, general)`) only works when the specific case is **semantically a refinement** of the general case. `ErrUserNotFound` and `ErrRoleNotFound` refine `ErrNotFound`, so joining is correct: a caller branching only on `ErrNotFound` still gets the right behaviour. But `ErrRoleDuplicate` is the inverse condition — the row was *found* and that is precisely the failure. Joining it with `ErrNotFound` would make `errors.Is(err, ErrNotFound)` true for a duplicate insert, which is a lie that any general-purpose 404 mapper would happily act on. Keep "found" sentinels (duplicate, conflict, already-exists) standalone; only "missing" sentinels get the `errors.Join` treatment.
