# Optimistic version concurrency for cross-request edits (vs. `FOR UPDATE` for in-tx guards)

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

This is the cross-request sibling of [TOCTOU authorization guard: lock the read rows with `FOR UPDATE`](toctou-authorization-guard-for-update-lock.md). Both protect `adminEditUser`, but against different concurrency hazards on different rows:

| Concern | Hazard | Mechanism | Row locked / versioned |
|---|---|---|---|
| Authorization guard (FOR UPDATE) | A concurrent rename/delete of the **role** commits between the in-tx guard read and the membership write | Pessimistic `FOR UPDATE` lock, within one transaction | role rows |
| Lost-update prevention (this doc) | Two admins load the **same user** and save concurrently; the second silently overwrites the first | Optimistic `version` token, across two requests | user row |

## Why optimistic, not pessimistic, for the user-row edit

The edit spans **two requests** with human think-time in between: a `GET` that loads the form, then a mutation that saves it. You cannot hold a `FOR UPDATE` lock across that gap — the lock lives and dies with a single transaction, and the read happens in a different request from the write. Pessimistic locking only closes a window *within* one transaction (which is exactly why the role guard uses it).

A `version` token round-tripped through the client is the only way to detect that a concurrent edit committed between the `GET` and the mutation: the client echoes the version it saw, and the write fails if the row has moved on.

## What

- **Migration.** `version bigint NOT NULL DEFAULT 0`. The default makes every existing row start at a valid token; `NOT NULL` keeps the compare-and-swap total. golang-migrate's pgx/v5 driver does **not** auto-wrap migrations, so the up/down SQL wraps the `ALTER TABLE` in explicit `BEGIN`/`COMMIT`.
- **Compare-and-swap UPDATE.** `UpdateTxVersioned` issues `UPDATE users SET <patch>, version = version + 1 WHERE id = ? AND version = ?`. The `version = version + 1` is added to the update map **unconditionally**, so the `SET` clause is never empty — `RowsAffected > 0` therefore reflects "the `WHERE` matched", not "the patch was non-empty".
- **Disambiguate `RowsAffected == 0` with a probe.** Zero rows affected conflates two cases. A follow-up `SELECT id WHERE id = ?` inside the same tx splits them: `gorm.ErrRecordNotFound` → `repository.ErrNotFound`; any other (`err == nil`) → `repository.ErrConcurrentUpdate`. A non-`ErrRecordNotFound` probe error is wrapped and returned, never misclassified as a conflict. The probe reads outside the CAS row's lock window, so a delete-between-update-and-probe race can report `ErrNotFound` instead of `ErrConcurrentUpdate` — benign, since both resolve to "edit failed, reload" on the client.
- **Always issue the versioned update, even for a role-only edit.** The pre-version code skipped the user `UPDATE` entirely when no profile field changed (a `profilePatch` bool). Routing **every** edit through `UpdateTxVersioned` is what makes the concurrency guard cover role-only edits too; the `updated_at` trigger correctly advances on the empty patch.
- **Surface the conflict as errors-as-data.** `ErrConcurrentUpdate` maps to a typed `ConcurrentUpdateError` variant on the `AdminEditUserResult` union (see [Result Union: errors as data](../error-wrapping/result-union-errors-as-data.md)), not a top-level `gqlerror`, so the client can branch on `__typename` to prompt "reload and try again".

Reference: `backend/internal/repository/user.go` (`UpdateTxVersioned`, `ErrConcurrentUpdate`), `backend/internal/usecase/admin_user.go` (`EditUser`, `AdminEditUserOutcome.ConcurrentUpdate`), `backend/internal/database/migrations/20260521090000_add_version_to_users.{up,down}.sql`, and the roundtrip proof in `backend/internal/database/users_version_roundtrip_test.go`.

The frontend side of the contract — how the client preserves the conflict banner across the auto-refetch it triggers — is in [Preserve a conflict banner across a same-entity refetch](../../frontend/typescript-conventions/preserve-banner-across-same-entity-refetch.md).
