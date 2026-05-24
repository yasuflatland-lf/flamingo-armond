# Transactional read-modify-write must use the `tx` handle, not `r.db`

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a usecase performs a read → compute → write sequence inside a `db.Transaction` closure, every database call in the closure — including the initial read — must use the `tx *gorm.DB` handle passed to that closure. Using the repository's base `r.db` handle for the read breaks snapshot isolation: a concurrent writer can commit between the non-transactional read and the subsequent write, and the stale value silently overwrites the newer one.

```go
// Wrong: read uses r.db (non-transactional) even though the write uses tx.
err = u.tx(ctx, func(tx *gorm.DB) error {
    existing, err := u.fsrsRepo.FindByUserAndCardIDs(ctx, userID, cardIDs) // r.db inside!
    if err != nil {
        return err
    }
    newState := compute(existing)
    return u.fsrsRepo.UpsertTx(ctx, tx, newState) // tx here
})

// Correct: both operations run on the same tx handle.
err = u.tx(ctx, func(tx *gorm.DB) error {
    existing, err := u.fsrsRepo.FindByUserAndCardIDsTx(ctx, tx, userID, cardIDs)
    if err != nil {
        return err
    }
    newState := compute(existing)
    return u.fsrsRepo.UpsertTx(ctx, tx, newState)
})
```

**How to expose a transactional read:** follow the shared-helper pattern described in [Tx and non-Tx repository methods share a private helper to avoid drift](repo-tx-and-nontx-share-private-helper.md). Add a `FindXxxTx(ctx, tx, ...)` method that delegates to a private `findXxxOn(db *gorm.DB, ...)` helper. The standalone `FindXxx` calls the helper with `r.db`; the transactional version calls it with `tx`. Both the interface consumed by the usecase and the narrow interface (`XxxRepoForYyy`) must declare the `Tx` variant so the closure can call it.

**Why this matters:** GORM's `r.db` handle opens a new connection on each call. Two concurrent swipes targeting the same `(userID, cardID)` row each read the current FSRS state independently, compute a new state, then both write. The second write sees the _original_ state rather than the result of the first swipe, silently discarding the scheduling update. The `tx` handle holds the transaction open and ensures the read is part of the same snapshot as the write.

Reference: `backend/internal/repository/user_card_fsrs.go` — `FindByUserAndCardIDs` (non-tx) and `FindByUserAndCardIDsTx` (tx variant). `backend/internal/usecase/swipe.go` — `HandleSwipe` uses `FindByUserAndCardIDsTx` inside its transaction closure.

## When the read informs an authorization decision, snapshot isolation is not enough

The rule above closes the window for two writers racing the **same** row: the `tx` handle keeps the read and write in one snapshot. When the in-tx read instead informs an *authorization or branching decision* — a check-then-act (TOCTOU) sequence — and the data read belongs to a *different* aggregate that a concurrent transaction can rename or delete, moving the read into the tx is necessary but **not** sufficient. Under Postgres `READ COMMITTED`, each statement re-snapshots, so a concurrent commit between the in-tx `SELECT` and the dependent write still slips through. There you must lock the read rows with `clause.Locking{Strength: "UPDATE"}`. See [TOCTOU authorization guard: lock the read rows with `FOR UPDATE`, abort via a control-flow sentinel](toctou-authorization-guard-for-update-lock.md).
