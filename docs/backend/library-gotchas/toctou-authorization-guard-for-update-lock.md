# TOCTOU authorization guard: lock the read rows with `FOR UPDATE`, surface guard outcomes via a captured variable

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

This is the authorization-guard extension of [Transactional read-modify-write must use the `tx` handle, not `r.db`](gorm-tx-read-modify-write.md). That rule covers a read → compute → write where two concurrent writers race the same row. This one covers a read that informs an **authorization or branching decision** which a later write depends on — a check-then-act (TOCTOU) sequence — where the data being read is owned by a *different* aggregate that a concurrent transaction can mutate.

For the complementary cross-request lost-update hazard on the same `adminEditUser` flow — two admins editing the same user across separate `GET`/mutation requests — see [Optimistic version concurrency for cross-request edits](optimistic-version-concurrency-cross-request.md). That uses an optimistic `version` token (not a `FOR UPDATE` lock) precisely because the hazard spans two requests with human think-time, where no transaction-scoped lock can reach.

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

## Returning a business outcome from a guard that runs before any write

When the caller is editing their own row (`callerID == id`), the guard sits at the front of the transaction. When it decides the edit must **not** proceed (self-demotion, or an unknown role id), it wants to return a *non-error* business outcome to the caller (a `CannotRevokeOwnAdmin` or a `Validation` field on the outcome struct) without performing any write. Because the guard runs **before** any `UpdateTxVersioned` or `SetUserRolesTx` call, returning `nil` from the closure commits an empty transaction — there is nothing to roll back.

The pattern captures the guard result in a variable declared outside the closure:

```go
var earlyOutcome *AdminEditUserOutcome

err = u.tx(ctx, func(tx *gorm.DB) error {
    if callerID == id {
        // ... look up roles with FOR UPDATE lock ...
        if len(roles) != len(roleIDs) {
            earlyOutcome = &AdminEditUserOutcome{Validation: NewInputValidationInfo("roleIds", "role not found")}
            return nil // commit empty tx; nothing was written
        }
        if !keepsAdmin {
            earlyOutcome = &AdminEditUserOutcome{CannotRevokeOwnAdmin: true}
            return nil // same: empty commit, real outcome in earlyOutcome
        }
    }
    // ... UpdateTxVersioned, SetUserRolesTx ...
    return nil
})

if earlyOutcome != nil {
    return *earlyOutcome, nil // checked before err
}
if err != nil { /* real infra/context errors */ }
```

**The Why:** because the guard runs before any write, a `return nil` from the closure is safe — GORM commits, but the transaction is empty. The real outcome travels in the closure-captured `earlyOutcome` pointer, checked before `err` in the post-tx code so it is never handed to the error-classification path. No control-flow sentinel is needed: the nil-commit approach avoids the rollback entirely, so there is no error for GORM to propagate. A non-nil `earlyOutcome` is the in-band signal that the guard fired; any non-nil `err` after that check is a real infrastructure or context error.

Reference: `backend/internal/usecase/admin_user.go` — `earlyOutcome`, the `EditUser` transaction closure, and the post-tx `if earlyOutcome != nil { return *earlyOutcome, nil }` check that runs before `if err != nil`.
