# `ON DELETE` FK action requires an integration test against a real database

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`ON DELETE CASCADE`, `ON DELETE SET NULL`, `ON DELETE RESTRICT`, and `ON DELETE NO ACTION` are declarative properties of the migration SQL. None of them are exercised by repository unit tests, GORM mocks, or schema introspection — the FK action is a server-side response to a `DELETE` statement, and the only way to verify it is to execute the `DELETE` against a real Postgres instance and assert the post-state.

A migration that meant to write `ON DELETE CASCADE` but landed `ON DELETE SET NULL` (or vice versa) compiles, applies cleanly, and passes every schema-assertion test. The mistake surfaces only when a user account is deleted in production — and by then the damage is already done.

## The pattern: insert parent + child, delete parent, assert child state

```go
func TestUserPreferenceRepository_OnDeleteUser_CascadesPreferenceRow(t *testing.T) {
    t.Parallel()
    ctx := context.Background()

    userID := insertAuthUser(t, ctx)
    cgID := insertCardgroupForUser(t, ctx, userID, "fk-cascade-user-cg")

    repo := repository.NewUserPreferenceRepository(testDB.GORM)
    if err := repo.UpsertLastViewedCardgroup(ctx, userID, cgID); err != nil {
        t.Fatalf("UpsertLastViewedCardgroup: %v", err)
    }
    if !prefRowExists(t, ctx, userID) {
        t.Fatal("expected user_preferences row to exist before user deletion")
    }

    // Delete from auth.users; the ON DELETE CASCADE FK on public.users(id) removes
    // the public.users row, which then cascades to user_preferences via its own
    // ON DELETE CASCADE FK.
    sqlDB := sqlDBHandle(t)
    if _, err := sqlDB.ExecContext(ctx,
        `DELETE FROM auth.users WHERE id = $1`, userID); err != nil {
        t.Fatalf("delete auth.users %q: %v", userID, err)
    }

    if prefRowExists(t, ctx, userID) {
        t.Fatal("user_preferences row still exists after user deletion: FK is not ON DELETE CASCADE")
    }
}
```

The same shape applies to `ON DELETE SET NULL`: insert the parent and child, delete the parent, assert the child's FK column is now `NULL`. For `ON DELETE RESTRICT`: insert parent and child, attempt to delete the parent, assert the `DELETE` fails with the expected SQLSTATE.

## Test the failure mode you would otherwise ship

Pair the assertion with a doc-comment that names the alternatives the test rules out — the failure mode you would otherwise ship:

```go
//   - RESTRICT would cause the auth.users DELETE to fail, breaking user
//     account deletion and causing a service outage.
//   - SET NULL would violate the PRIMARY KEY constraint on user_preferences.user_id
//     (NOT NULL is implied by PRIMARY KEY), crashing the database operation.
```

The comment turns a green test into a regression signal: a reviewer scanning the test sees both the intended FK action and the two adjacent FK actions that would have produced visible outages. Without the comment, the test reads as "we deleted a row and another row also went away" and the next contributor cannot recover the intent.

## Place the test alongside the repository, not in the migration test

The migration test (covered separately in [Migration down/up roundtrip test](migration-down-up-roundtrip-test.md)) is responsible for the schema shape; the FK-action test is responsible for the runtime behaviour of `DELETE`. Splitting them keeps each test focused on one failure surface: a schema regression breaks the migration test, an FK-action regression breaks the repository test. Combining them produces a single test that fails for two distinct reasons, which is harder to triage.

## Reference

`backend/internal/repository/user_preference_test.go` carries both `TestUserPreferenceRepository_OnDeleteUser_CascadesPreferenceRow` (parent-user delete cascades the preference row) and `TestUserPreferenceRepository_OnDeleteCardgroup_SetsNull` (cardgroup delete nulls the `last_viewed_cardgroup_id` column). The migration that declares both FK actions is `20260516120000_extract_user_preferences.up.sql`.
