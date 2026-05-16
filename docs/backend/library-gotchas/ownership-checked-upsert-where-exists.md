# Ownership-checked UPSERT: collapse `EXISTS` ownership predicate into the INSERT statement

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A "verify the user owns the parent row, then write the child row" sequence written as two SQL statements carries a TOCTOU race: an admin or concurrent operation can delete or re-assign the parent between the ownership check and the write. The right shape is to fold the ownership predicate into the write statement itself — one round-trip, no race window, and the same predicate guards both inserts and updates.

```sql
INSERT INTO user_preferences (user_id, last_viewed_cardgroup_id, updated_at)
SELECT ?, ?, now()
WHERE EXISTS (
    SELECT 1 FROM cardgroups WHERE id = ? AND owner_id = ?
)
ON CONFLICT (user_id) DO UPDATE
SET last_viewed_cardgroup_id = EXCLUDED.last_viewed_cardgroup_id,
    updated_at               = EXCLUDED.updated_at;
```

Three properties are load-bearing:

1. **`INSERT ... SELECT ... WHERE EXISTS (...)`**, not `INSERT ... VALUES (...)`. The `SELECT ... WHERE EXISTS` form lets the same statement evaluate the ownership predicate; `VALUES` cannot carry a `WHERE` clause. When the predicate is false, the `SELECT` returns zero rows, the `INSERT` writes nothing, and `RowsAffected == 0`.
2. **`ON CONFLICT (user_id) DO UPDATE`** is only reached when the `SELECT` produces a row. A failed ownership check yields zero rows, so the `ON CONFLICT` branch never fires for a non-owned target — the existing preference row is left untouched. The single statement encodes the rule "no ownership ⇒ no write" for both first-write and update cases.
3. **`RowsAffected == 0` is the only "not found / not owned" signal.** The caller cannot distinguish the two cases by the rowcount — both surface as zero. That is the intended posture: collapsing both into one sentinel (`ErrCardgroupNotFound`) avoids an existence oracle. See [`docs/backend-graphql.md` § "Existence-oracle prevention via collapsed `BAD_USER_INPUT`"](../../backend-graphql.md#existence-oracle-prevention-via-collapsed-bad_user_input).

## TOCTOU race-recovery via the `23503` classifier

The `EXISTS` subquery and the `INSERT` are inside one statement, but Postgres still re-checks the FK at commit time. If the cardgroup row is deleted by a concurrent transaction between the `EXISTS` evaluation and the FK enforcement, Postgres returns SQLSTATE `23503` (foreign key violation) on `last_viewed_cardgroup_id`. The classifier maps this back to the same `ErrCardgroupNotFound` sentinel so the caller sees one error shape, not two:

```go
func classifyUserPreferenceCardgroupFKError(err error) error {
    var pgErr *pgconn.PgError
    if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
        return nil
    }
    if strings.Contains(pgErr.ConstraintName, "last_viewed_cardgroup_id") {
        return ErrCardgroupNotFound
    }
    return nil
}
```

This matches the [Postgres FK violation classification](../error-wrapping/postgres-fk-violation-23503.md) rule and ensures the race-deleted-parent case is reported as a client-fixable `BAD_USER_INPUT`, not a server-side `INTERNAL`.

## When this pattern applies

Any "child row written by the caller, parent row owned by the caller" relationship qualifies. The same shape works for `user_settings`, audit-trail bookmarks, or any per-user denormalised state that must remain consistent with an ownership predicate. The pattern is a strictly better default than the older `UPDATE ... WHERE EXISTS (...)` form for two reasons: it supports first-write (no row yet) without a second `INSERT ... ON CONFLICT` statement, and it is symmetric across first-write and update so a single test fixture covers both.

## Reference

`backend/internal/repository/user_preference.go` — `UpsertLastViewedCardgroup` issues the SQL above; `classifyUserPreferenceCardgroupFKError` covers the race-deletion case. `backend/internal/repository/user_preference_test.go` exercises the missing-cardgroup, non-owned, and FK-race branches; the GraphQL surface this enables is `setLastViewedCardgroup(cardgroupId: ID!): User!` (see [`docs/backend-graphql.md` § "`setLastViewedCardgroup` and `User.lastViewedCardgroup`"](../../backend-graphql.md#setlastviewedcardgroup-and-userlastviewedcardgroup)).
