# Layered sentinels via `errors.Join`

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

When introducing a more specific sentinel alongside an existing general one (e.g. adding `ErrUserNotFound` while `ErrNotFound` is still in use across the repository), return `errors.Join(specific, general)` from the new call site. Callers that already match `errors.Is(err, ErrNotFound)` keep working; callers that want the finer split can branch on the specific sentinel first. This avoids a flag-day rename across every caller and lets the finer sentinel migrate in at its own pace. **Always check the more specific sentinel before the general one** — `errors.Is` returns true for both, so reversing the order silently routes user-not-found into a generic 404 path.
