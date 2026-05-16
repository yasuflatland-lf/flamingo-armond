# RLS `INSERT-own` assertion on a PK-keyed table needs a fixture row that does not exist yet

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A row-level-security regression test for a `FOR INSERT WITH CHECK (user_id = auth.uid())` policy looks like it asserts "user A can insert a row for themselves". That assertion silently degrades when the test fixture already inserted a row for user A and the `INSERT` statement uses `ON CONFLICT (user_id) DO NOTHING` — the policy is **never exercised** because the conflict short-circuits before the `WITH CHECK` clause runs.

```sql
-- Fixture step (setup):
INSERT INTO public.user_preferences (user_id, last_viewed_cardgroup_id)
VALUES ($1, $2);                          -- userA already has a row.

-- Test step (the supposedly INSERT-own assertion):
INSERT INTO public.user_preferences (user_id, last_viewed_cardgroup_id)
VALUES ($1, $2)
ON CONFLICT (user_id) DO NOTHING;          -- 0 rows affected, RLS not invoked.
```

The assertion `assertRows(..., 1)` then fails — but the failure looks like an RLS denial, when in reality the policy was never tested. The opposite shape (`assertRows(..., 0)` if the test was negative) silently passes for the wrong reason.

## The fix: reserve a fresh user for INSERT-own assertions

Add a dedicated user (e.g. `userC`) to the RLS fixture that the setup steps **never** insert a preference row for. Use that user when the test needs to assert "an authenticated caller can write their own row":

```go
type rlsFixture struct {
    userA     string
    userB     string
    userC     string // fresh user with no user_preferences row; used for INSERT-own tests
    adminUser string
    // ...
}

func createRLSFixture(...) rlsFixture {
    // ...
    userC := insertRLSAuthUser(t, ctx, sqlDB) // reserved for INSERT-own tests; no pre-inserted rows
    // ...
}
```

The INSERT-own assertion then targets `userC`, where the `ON CONFLICT` branch cannot fire:

```go
// INSERT-own: User C (no pre-existing row) can insert a row for themselves.
// userA/userB already have rows from the fixture setup above, so we use userC
// to avoid the ON CONFLICT DO NOTHING returning 0 rows affected.
assertRows(t, execOKAs(t, ctx, authPool, fx.userC, insertUserPreferencesSQL(), fx.userC), 1)

// INSERT-other denied: User C cannot insert a row with User A's user_id.
execDeniedAs(t, ctx, authPool, fx.userC, insertUserPreferencesSQL(), fx.userA)
```

## Why this is per-aggregate, not a universal RLS pattern

The conflict happens because `user_preferences.user_id` is a `PRIMARY KEY` (1:1 with `users`). On a multi-row-per-user table (e.g. `cards`, `swipes`, `audit_log`), the same fixture user can host both setup rows and an INSERT-own assertion row without an `ON CONFLICT` interaction — the second `INSERT` simply adds a new row. The conflict-only-hides-RLS hazard is specific to **1:1 PK-keyed sibling aggregates** (preferences, settings, per-user singletons) and any other table whose key matches the per-user fixture insertion.

Audit every RLS test that asserts an `INSERT-own` outcome on such a table: if the fixture setup already populated a row for the target user, the test is testing the conflict branch, not the policy.

## Reference

`backend/internal/database/rls_test.go` reserves `userC` for `user_preferences` INSERT-own assertions and uses `userA` / `userB` for SELECT, UPDATE, DELETE policy tests (where the pre-existing rows are required). The matching policy declarations live in `backend/internal/database/migrations/20260516120000_extract_user_preferences.up.sql` — four policies (`select_own_or_admin`, `insert_own_or_admin`, `update_own_or_admin`, `delete_own_or_admin`).
