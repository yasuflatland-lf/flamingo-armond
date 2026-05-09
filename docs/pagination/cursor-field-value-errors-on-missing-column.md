# `cursorFieldValue` errors on missing column

> Part of the [pagination](../../.claude/rules/pagination.md) rules. Cross-referenced by `docs/backend.md` and `docs/frontend.md`.

A silent zero-value fallback (e.g. `time.Time{}`) would generate a wrong-but-valid SQL predicate and quietly skip rows. The usecase hydrates the column required by the active `orderBy` before calling the repository, so a missing column is a caller bug — surface it as `gqlerr.Internal` rather than return wrong rows.
