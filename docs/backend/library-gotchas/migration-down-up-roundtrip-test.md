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

## `Steps(-N)` is relative to latest — a newer migration silently retargets it

`m.Steps(-1)` rolls back **one migration relative to the current latest version**, not relative to a named migration. A roundtrip test for migration X that uses `Steps(-1)` correctly targets X only while X is the newest migration. The moment a newer migration Y is added, `Steps(-1)` rolls back Y instead, and X's assertions (e.g. `requireColumnMissing` for X's column) fail because X is still applied — the schema no longer matches the expected rolled-back state.

Concretely: adding the `cards.position` migration made it the newest, so `TestUsersVersionDownUpRoundtrip` — which used `Steps(-1)` expecting to drop `users.version` — began rolling back `cards.position` and left `users.version` in place. The fix was `Steps(-2)` to roll back past both Y and X, then `Steps(1)` to re-apply only X; `t.Cleanup` restores to latest. See `backend/internal/database/users_version_roundtrip_test.go`.

**Operator instruction:** when you add a new migration, audit every existing down/up roundtrip test that steps by relative count:

```bash
grep -rn "Steps(-1)" backend/internal/database
```

For each hit, check whether a newer migration now sits above the one under test. If so, bump the rollback count (e.g. `-1` → `-2`) or add a preliminary `Steps(-1)` to step past the newer migration first, then restore with a matching extra `Steps(1)` in `t.Cleanup`. A more robust alternative is to migrate down to a named version rather than by relative count, but the existing tests use relative counts.

This failure mode is the migration-ordering analogue of the broader "adjacent edit surfaces a pre-existing assumption" theme in [`.claude/rules/scope-discipline.md`](../../../.claude/rules/scope-discipline.md).

## Bump every per-migration `Steps(-N)` when a newer migration lands

The relative-count trap above is a standing tax: **each per-migration roundtrip test hardcodes a step count `N` that counts every migration newer than its target, so a migration added anywhere above the target (including at HEAD) shifts the distance and `N` must increase by 1 in each affected test.** These tests migrate to HEAD, `m.Steps(-N)` down past their target to assert the column/table is gone, then `m.Steps(1)` to re-apply.

The failure mode when you forget: the down navigation stops one migration short of the target, the column is still present, and the test fails with `expected column X.Y to be absent`.

Worked example: adding `20260614000000_add_master_tables` (the Master Catalog tables) shifted both per-migration tests up by one — `TestCardsPositionDownUpRoundtrip` went `Steps(-4)` → `Steps(-5)` and `TestUsersVersionDownUpRoundtrip` went `Steps(-5)` → `Steps(-6)`. Adding `20260615000000_add_learn_display_mode_to_user_preferences` directly on top shifted both again — `TestCardsPositionDownUpRoundtrip` to `Steps(-6)` and `TestUsersVersionDownUpRoundtrip` to `Steps(-7)`. See `backend/internal/database/cards_position_roundtrip_test.go` and `users_version_roundtrip_test.go`.

The newest migration's **own** roundtrip test is the stable anchor: it uses `Steps(-1)` because its target *is* HEAD, and it stays at `-1` until a further migration is added on top of it. `TestLearnDisplayModeDownUpRoundtrip` (`backend/internal/database/learn_display_mode_roundtrip_test.go`) added with the `20260615000000_add_learn_display_mode_to_user_preferences` migration uses `Steps(-1)` and needs no bump — while the same migration forced its two older siblings (`cards_position`, `users_version`) up by one. The asymmetry is the whole point: a new migration leaves its own `-1` test alone and shifts every older sibling's count up by exactly one.

The **generic** `TestMigrateDownUpRoundtrip` (`backend/internal/database/migrate_roundtrip_test.go`) is immune to step-count drift — it uses a full `m.Down()` / re-`Up`, not a relative count. But it hardcodes a `want` list of expected public tables, so **any new table must be appended to that list** (`master_cardgroups` and `master_cards` were added for the master-tables migration; `add_learn_display_mode` adds only a column, so it does not touch the `want` list).

The harness to run when adding ANY migration:

```bash
# Bump each per-migration count by 1 …
grep -rn 'm.Steps(-' backend/internal/database/*_test.go
# … and append any new table(s) to the `want` slice in:
#   backend/internal/database/migrate_roundtrip_test.go
```

**CI-only nuance.** These are testcontainers integration tests that require a running Postgres. A step-count regression is **invisible** to local `go build` / `go vet` (which only compile) and cannot be caught locally when Docker is unavailable — it manifests only when the migration actually runs (CI). A green local compile does **not** prove a migration change is correct; push to CI to verify. The testcontainers / Docker-unavailable constraint is the same one noted in [`docs/backend-db.md` § "Migration test quality bar"](../../backend-db.md#migration-test-quality-bar).

## Restore the suite state

Both tests must end with the database migrated back up. A test that leaves the schema rolled back contaminates every downstream test in the same package. The pattern above re-applies the up migration at the end of each test; alternative shapes use `t.Cleanup` to centralise the restore.

## Reference

`backend/internal/database/extract_user_preferences_test.go` contains `TestExtractUserPreferences_DownUpRoundtrip` and `TestExtractUserPreferences_NullPrefHandledByDown`. The migration pair under test is `20260516120000_extract_user_preferences.{up,down}.sql`. The general migration mechanics (filename format, `schema_migrations.dirty` recovery, BEGIN/COMMIT requirement for the pgx driver) are documented in [`docs/backend-db.md` § "Migrations"](../../backend-db.md#migrations).
