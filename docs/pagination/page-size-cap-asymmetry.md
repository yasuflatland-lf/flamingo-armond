# Page-size cap asymmetry (`maxPageSize` vs `pageCap`)

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

Two constants work together:

- `maxPageSize = 100` — user-facing cap enforced by the usecase.
- `pageCap = maxPageSize + 1` (i.e. 101) — repository-level limit that lets the `+1` trick survive a request at the documented maximum.

Document both constants. Changing one without the other silently caps a layer below spec.
