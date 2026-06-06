# SQL comparison on a nullable LEFT JOIN column silently excludes NULL rows

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A comparison predicate (`>=`, `<`, `<=`, `>`, `=`) against a nullable column
evaluates to `UNKNOWN`, not `TRUE`, when the column is NULL — SQL three-valued
logic. The `WHERE` clause keeps only rows where the predicate is `TRUE`, so
every NULL-column row is silently dropped without a `IS NOT NULL` filter ever
being written. On a LEFT JOIN this is the usual case: rows with no matching
right-side row carry NULL in every projected right-side column.

The trap is that the exclusion is *invisible* in the SQL text. A reviewer
scanning `WHERE ucs.last_review >= ?` sees no NULL handling and may "fix" it by
adding `OR ucs.last_review IS NULL`, which would be wrong here — or, conversely,
may not realize the NULL rows are gone and reason about them as if they were
included.

## Worked example

`findPracticeCardsOn` (`backend/internal/repository/card.go`) selects the
FSRS-safe practice pool — cards the user already reviewed at or after the
start-of-day boundary:

```go
rows, err := dueRowsOn(db, userID,
    "cards.cardgroup_id = ? AND ucs.last_review >= ?",
    []any{cardgroupID, reviewedAfter},
    "random()",
    limit,
    "repository: card: find practice cards")
```

`ucs` is the `user_card_fsrs` row joined via `LEFT JOIN`. A never-reviewed card
has no FSRS row, so `ucs.last_review` is NULL and `NULL >= ?` is `UNKNOWN` — the
row is excluded. That is exactly the intended behavior: a card that has never
been reviewed cannot have been "reviewed today", so it must not enter the
practice pool. The exclusion is deliberate, **not** a missing `IS NOT NULL`
guard.

## Document the deliberate exclusion in code

Because the guard is implicit, an in-code comment must state that the NULL
exclusion is intended, so a future reader does not add a defensive
`IS NOT NULL` or treat the drop as a bug. `findPracticeCardsOn` carries:

```go
// NULL last_review (never-reviewed cards) can never satisfy `>=`, so no
// `IS NOT NULL` guard is needed.
```

The complementary learn window (`findDueCardsOn`) makes its own NULL handling
*explicit* with `ucs.due IS NOT NULL` precisely because there the new-card path
is a separate window that wants those NULL rows — see the boundary-pin test in
[`strict-cutoff-boundary-fixture-and-mutation-proof.md`](strict-cutoff-boundary-fixture-and-mutation-proof.md).
The shared SELECT/JOIN that makes both windows use the identical projection is
`dueRowsOn`; see [`repo-tx-and-nontx-share-private-helper.md`](repo-tx-and-nontx-share-private-helper.md).
