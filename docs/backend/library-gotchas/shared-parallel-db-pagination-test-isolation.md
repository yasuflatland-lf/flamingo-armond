# Shared parallel test DB: isolate no-cursor pagination queries

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

The repository integration tests run with `t.Parallel()` against a single
testcontainers Postgres provisioned once in `TestMain`. Every test's fixture
rows coexist in that one database for the duration of the run. A repository
query that is **not** scoped to the test's own rows therefore sees every other
parallel test's rows too.

This is a silent trap specifically for a **no-cursor** `first=N` / `last=N`
pagination call. Such a call returns the `N` globally-ordered rows for the whole
table, not the test's rows. When another parallel test has inserted rows that
sort ahead of this test's fixtures (e.g. a lower `sort_order`, an earlier
`created_at`), the page comes back full of other tests' rows and the test's own
rows are absent — the assertion fails intermittently or always, depending on
insertion order.

```go
// FLAKY: first=2 with no cursor returns the two globally-lowest sort_order rows,
// which other parallel tests' published rows can occupy.
fwd, _ := repo.FindPublishedPage(ctx, nil, nil, 2, 0, OrderBySortOrder, Asc, nil)
ours := filterByIDs(fwd, ourIDs)   // often empty — fwd holds other tests' rows
require.GreaterOrEqual(t, len(ours), 1)   // fails
```

Two isolation techniques, both already used in
`backend/internal/repository/master_catalog_test.go`:

1. **Name-search predicate.** Give every fixture row a name ending in a
   per-test `base := uuid.NewString()` and pass `search := base` to the query.
   The `ILIKE %base%` predicate narrows the result to exactly this test's rows,
   so even `first=2` deterministically returns the test's two lowest rows. This
   is the fix applied to `FindPublishedPage_ForwardAndBackward` and the pattern
   `CountPublished_ExcludesDraft` was written with from the start.
2. **Cursor anchored to a fixture row.** A query with an `after`/`before` cursor
   pinned to one of the test's own rows windows the result around that row, which
   already excludes most foreign rows; combine with a `filterByIDs` pass to drop
   any stragglers that share the cursor's order-field value.

The `filterByIDs(page, ourIDs)` helper alone is **not** sufficient for a
no-cursor `first=N` query: it can only filter what the page contains, and the
page may contain zero of the test's rows. Filtering is a defensive post-step;
the isolation must happen in the query (search or cursor) so the test's rows are
actually in the returned window.

**Reference:** `backend/internal/repository/master_catalog_test.go` —
`TestMasterCardgroupRepository_FindPublishedPage_ForwardAndBackward` (search
isolation), `TestMasterCardgroupRepository_CountPublished_ExcludesDraft`
(search isolation), and the `filterCatalogByIDs` / `catalogIDs` helpers.
