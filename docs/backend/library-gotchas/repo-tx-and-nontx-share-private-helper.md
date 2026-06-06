# Tx and non-Tx repository methods share a private helper to avoid drift

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a repository needs to expose both a standalone version and a transactional
version of the same query, do not copy the body. Extract a private helper that
accepts `*gorm.DB` directly and call it from both public methods:

```go
// backend/internal/repository/card.go

// FindByID is the standalone version — uses the repo's default DB handle.
func (r *cardRepo) FindByID(ctx context.Context, id string) (*domain.Card, error) {
    return findCardByID(ctx, r.db, id)
}

// FindByIDTx is the transaction-aware version — operates on the caller's tx,
// adding a row lock so the caller can read-modify-write inside the transaction.
func (r *cardRepo) FindByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error) {
    return findCardByID(ctx, tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
}

// findCardByID holds the shared implementation.
// db is either the default handle (from FindByID) or a transaction handle
// (from FindByIDTx) — the lock clause is applied by the caller, not here.
func findCardByID(ctx context.Context, db *gorm.DB, id string) (*domain.Card, error) {
    var row gormCard
    if err := db.WithContext(ctx).Where("id = ?", id).Take(&row).Error; err != nil {
        // ... map gorm.ErrRecordNotFound to repository.ErrNotFound, wrap others ...
    }
    // ... build *domain.Card from row ...
}
```

**Why this matters:** a copy-pasted body will eventually drift. The standalone
version might receive an index hint or a filter clause that the Tx version misses,
or the Tx version picks up a bug fix that the standalone version never gets. The
shared helper is the single source of truth — fix it once, both callers benefit.

**Naming convention:** the private helper is named after the operation,
accepting `*gorm.DB` as one of its leading arguments. Two naming forms coexist
in `card.go`: the `find<Aggregate>ByID` form shown above (`findCardByID`), and
an `On`-suffix form (`findDueCardsOn`) used when the operation name alone would
be ambiguous — the `On` suffix reads as "run this query *on* the supplied
handle".

**Lock placement is the caller's job.** `FindByIDTx` adds
`clause.Locking{Strength: "UPDATE"}` before delegating; the private helper never
applies a lock of its own. Keeping the lock at the public Tx method lets the
standalone `FindByID` stay lock-free while the transactional path acquires the
row lock the caller needs for a read-modify-write. Note also that
[GORM scan targets with embedded TableName methods](gorm-embedded-tablename-scan-confusion.md)
can silently break LEFT JOIN projection, so a helper that scans a joined query
should use a flat scan target rather than embedding `gormCard`.

## The same helper pattern extends to sibling window queries

The drift-avoidance argument is not limited to a Tx / non-Tx pair. It applies to
any set of queries that must share a SELECT/JOIN so their projection can never
diverge. `findDueCardsOn` (the learn window) and `findPracticeCardsOn` (the
inverse practice window) both call the same `dueRowsOn` helper, which owns the
`SELECT cards.* , ucs.state, ucs.due` column list and the `LEFT JOIN
user_card_fsrs ucs` clause. Each window passes its own WHERE predicate, ORDER
BY, and LIMIT; the columns and join can never drift between them because there
is one source. If the practice window inlined its own copy of the SELECT, a
later column addition to the learn window would silently skip practice.

**A shared helper serving callers with different wrap prefixes takes the
message as a parameter.** `dueRowsOn` is called by two windows whose
`eris.Wrap` layer prefixes differ (`repository: card: find due cards` vs.
`repository: card: find practice cards`). A shared helper must not hardcode a
fixed prefix — that would displace the caller-specific module attribution from
the error chain. The prefix travels as a `wrapMsg` argument the caller supplies:

```go
func dueRowsOn(db *gorm.DB, userID, where string, whereArgs []any, order string, limit int, wrapMsg string) ([]dueCardRow, error) {
    // ... run the shared LEFT JOIN query ...
    if err := db. /* ... */ .Find(&rows).Error; err != nil {
        return nil, eris.Wrap(err, wrapMsg)
    }
    return rows, nil
}
```

See [`error-classifier-helper-pass-through-with-caller-prefix.md`](../error-wrapping/error-classifier-helper-pass-through-with-caller-prefix.md)
for the full caller-supplied-prefix pattern and its double-wrap failure mode.

**Reference:** `backend/internal/repository/card.go` — `FindByID`,
`FindByIDTx`, and `findCardByID`. The `On`-suffix variant of the same idea is
`findDueCardsOn` (the shared body behind `FindDueCardsForUser`) and
`findPracticeCardsOn` (behind `FindPracticeCardsForUser`); both delegate to the
shared `dueRowsOn`.
