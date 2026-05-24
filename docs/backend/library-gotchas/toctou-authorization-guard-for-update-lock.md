# TOCTOU authorization guard: lock the read rows with `FOR UPDATE`, abort via a control-flow sentinel

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

This is the authorization-guard extension of [Transactional read-modify-write must use the `tx` handle, not `r.db`](gorm-tx-read-modify-write.md). That rule covers a read → compute → write where two concurrent writers race the same row. This one covers a read that informs an **authorization or branching decision** which a later write depends on — a check-then-act (TOCTOU) sequence — where the data being read is owned by a *different* aggregate that a concurrent transaction can mutate.

## Why moving the read into the tx is necessary but not sufficient

A self-demotion guard for `adminEditUser` must read the names of the submitted roles to decide whether the caller keeps the admin role, then write the new role membership. The first instinct — move the role-name read inside the transaction so it shares the write's snapshot — does not close the window under Postgres's default `READ COMMITTED` isolation. `READ COMMITTED` takes a **fresh snapshot at the start of each statement**, not once per transaction. A concurrent admin who renames or deletes the admin role and commits *between* the in-tx `SELECT` and the membership write is visible to the write; the guard's decision is already made against the stale name, and the demotion slips through.

The fix is to lock the read rows so the concurrent mutation cannot commit until the guard's transaction commits:

```go
q := db.WithContext(ctx).Where("id IN ?", ids)
if lock {
    q = q.Clauses(clause.Locking{Strength: "UPDATE"}) // FOR UPDATE
}
```

`FOR UPDATE` blocks any other transaction that tries to `UPDATE`/`DELETE`/lock the matched rows until this transaction ends. The guard's read and the role-set write are now atomic with respect to a concurrent rename/delete.

**The What:** when a read informs an authorization or branching decision that a subsequent write in the same transaction depends on, lock the read rows (`clause.Locking{Strength: "UPDATE"}`) inside that transaction — do not rely on snapshot isolation alone under `READ COMMITTED`.

Precedent in this repo: `backend/internal/repository/card.go` — `FindByIDTx` acquires `clause.Locking{Strength: "UPDATE"}`, proven by `backend/internal/repository/card_test.go` (`TestCardRepository_FindByIDTx_LocksRowForUpdate`, which asserts a second `FOR UPDATE NOWAIT` fails). The role analogue is `backend/internal/repository/role.go` — `FindByIDsTx` delegates to `findRolesByIDs(..., lock=true)`. The narrow interface consumed by the usecase (`backend/internal/usecase/admin_user.go`) must also declare the `Tx` variant so the closure can call it; see the shared-helper pattern in [Tx and non-Tx repository methods share a private helper to avoid drift](repo-tx-and-nontx-share-private-helper.md).

## Aborting the tx while returning a business outcome — the control-flow sentinel

The guard sits at the front of the transaction. When it decides the edit must **not** proceed (self-demotion, or an unknown role id), it must roll back — no write — yet return a *non-error* business outcome to the caller (a `CannotRevokeOwnAdminRoleError` or an `InputValidationError` variant). GORM's `db.Transaction(fn)` rolls back exactly when `fn` returns a non-nil error, and returns that error **verbatim** to the caller. So there is no in-band way to say "roll back, but the result is a success-shaped outcome."

The pattern uses an out-of-band channel:

1. A package-private control-flow sentinel — plain `errors.New`, never `eris` — returned from the closure to force the rollback:
   ```go
   var errGuardAbort = errors.New("usecase: admin user edit: guard abort")
   ```
2. The real outcome captured in a closure variable (`guard`), with a flag (`guardHit`) set at every `return errGuardAbort` site.
3. **Critically, the caller checks the captured-outcome flag before the tx error**, so the sentinel never reaches the error-classification path:
   ```go
   err = u.tx(ctx, func(tx *gorm.DB) error {
       // ... guard sets guard = ...; guardHit = true; return errGuardAbort
   })
   if guardHit {
       return guard, nil // sentinel consumed here; never classified
   }
   if err != nil { /* real infra/context errors */ }
   ```

**The Why:** because GORM hands the closure's error back verbatim on rollback, the real result needs a side channel (closure capture). The ordering invariant — `guardHit` checked before `err` — is what keeps the sentinel out of the error path (`gqlerr.FromUsecaseError`), where a plain `errors.New` with no `extensions.code` would misclassify as `INTERNAL` and 5xx the request. Using a plain `errors.New` rather than an `eris` wrap keeps the sentinel cheap and identity-matchable; it is deliberately not part of the wire-error vocabulary because it is never meant to reach the wire.

Reference: `backend/internal/usecase/admin_user.go` — `errGuardAbort`, the `EditUser` transaction closure, and the post-tx `if guardHit { return guard, nil }` check that runs before `if err != nil`.
