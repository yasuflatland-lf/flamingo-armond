# Migration down/up roundtrip test: prove the reverse path preserves data

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`golang-migrate` will happily apply a down migration that drops data the up migration would have backfilled — there is no built-in check that down + up restores the original state. A down migration tested only "does it run without error" is not enough: a missing `UPDATE` in the down direction silently turns every existing user's preference into `NULL` the first time an operator rolls back, and the loss is permanent because the down step deletes the source table on its way out.

The defensive shape is a **roundtrip test** that exercises both halves of the migration against the same fixtures and asserts the data survives.

## The pattern: seed at HEAD, down one step, assert, up one step, assert

```go
func TestExtractUserPreferences_DownUpRoundtrip(t *testing.T) {
    ctx := context.Background()
    db := openMigratedDB(t)
    defer db.Close()
    sqlDB := sqlDBForTest(t, db)

    // Precondition: fully migrated up — the new shape is live.
    require.True(t, tableExists(t, ctx, sqlDB, "user_preferences"))
    require.False(t, columnExists(t, ctx, sqlDB, "users", "last_viewed_cardgroup_id"))

    // Seed: user A has a preference; user B does not.
    userA := insertUserPrefAuthUser(t, ctx, sqlDB)
    userB := insertUserPrefAuthUser(t, ctx, sqlDB)
    cardgroupC := insertCardgroupForPref(t, ctx, sqlDB, userA)
    insertPref(t, ctx, sqlDB, userA, cardgroupC)

    // --- Down migration ---
    require.NoError(t, database.MigrateStepsForTest(testDSN, -1))
    require.True(t, columnExists(t, ctx, sqlDB, "users", "last_viewed_cardgroup_id"))
    require.Equal(t, cardgroupC, queryUUID(t, ctx, sqlDB, ..., userA))   // A backfilled
    require.False(t, queryNonNull(t, ctx, sqlDB, ..., userB))             // B stays NULL
    require.False(t, tableExists(t, ctx, sqlDB, "user_preferences"))

    // --- Up migration (re-apply) ---
    require.NoError(t, database.MigrateStepsForTest(testDSN, 1))
    require.True(t, tableExists(t, ctx, sqlDB, "user_preferences"))
    require.Equal(t, cardgroupC, queryUUID(t, ctx, sqlDB, ..., userA))   // A re-hydrated
    require.False(t, columnExists(t, ctx, sqlDB, "users", "last_viewed_cardgroup_id"))
    require.Equal(t, 4, rlsPolicyCount(t, ctx, sqlDB, "user_preferences")) // RLS re-applied
}
```

Three assertion families are load-bearing:

1. **Data survives the roundtrip.** Every seeded value is checked after the down step and again after the up step. A silent data loss in either direction surfaces here.
2. **Absent values stay absent.** User B (no preference) must come back through the roundtrip with no preference — a down migration that defaulted `last_viewed_cardgroup_id = some-arbitrary-cg` would be caught by this assertion.
3. **Auxiliary objects (indexes, RLS policies) re-apply on the up step.** The re-up runs the same up SQL as a fresh boot; a dropped policy or index in the down path that the up path then forgets to re-create silently disables security or wrecks query plans. Count the policies / indexes explicitly.

## Cover the NULL-pref edge case separately

A `user_preferences` row whose `last_viewed_cardgroup_id` was nulled by `ON DELETE SET NULL` is a legitimate state but must not feed into the down migration as a value. A second, smaller test seeds only a NULL-pref row and asserts the down migration leaves `users.last_viewed_cardgroup_id` NULL — never overwrites an existing NULL with the literal string `"NULL"` or a default cardgroup ID:

```go
func TestExtractUserPreferences_NullPrefHandledByDown(t *testing.T) {
    // ... insert user with user_preferences row where last_viewed_cardgroup_id IS NULL ...
    require.NoError(t, database.MigrateStepsForTest(testDSN, -1))
    _, hasValue := queryNullableUUID(t, ctx, sqlDB, "SELECT ... users.last_viewed_cardgroup_id ...", userC)
    require.False(t, hasValue, "null pref must not overwrite users column")
    // Restore the suite for downstream tests.
    require.NoError(t, database.MigrateStepsForTest(testDSN, 1))
}
```

A roundtrip on a NULL-pref row is the migration analogue of "the empty string is also a valid value" — most data-shape bugs are at the boundary, not in the middle.

## Restore the suite state

Both tests must end with the database migrated back up. A test that leaves the schema rolled back contaminates every downstream test in the same package. The pattern above re-applies the up migration at the end of each test; alternative shapes use `t.Cleanup` to centralise the restore.

## Reference

`backend/internal/database/extract_user_preferences_test.go` contains `TestExtractUserPreferences_DownUpRoundtrip` and `TestExtractUserPreferences_NullPrefHandledByDown`. The migration pair under test is `20260516120000_extract_user_preferences.{up,down}.sql`. The general migration mechanics (filename format, `schema_migrations.dirty` recovery, BEGIN/COMMIT requirement for the pgx driver) are documented in [`docs/backend-db.md` § "Migrations"](../../backend-db.md#migrations).
