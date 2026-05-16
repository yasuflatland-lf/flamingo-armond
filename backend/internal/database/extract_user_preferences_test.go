package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"backend/internal/database"
)

// extractUserPreferencesVersion is the integer timestamp of the
// 20260516120000_extract_user_preferences migration pair.
const extractUserPreferencesVersion int = 20260516120000

// predecessorOfExtractUserPreferences is the version of the last successfully
// applied migration before 20260516120000. Used to recover from a dirty-state
// after a failed up-migration during the roundtrip test.
const predecessorOfExtractUserPreferences int = 20260514000000

// tableExists reports whether a table with the given name exists in the
// public schema.
func tableExists(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName string) bool {
	t.Helper()
	var count int
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name   = $1
	`, tableName).Scan(&count); err != nil {
		t.Fatalf("query information_schema.tables for %q: %v", tableName, err)
	}
	return count == 1
}

// columnExists reports whether a column exists on the given table in the
// public schema.
func columnExists(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName, columnName string) bool {
	t.Helper()
	var count int
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name   = $1
		  AND column_name  = $2
	`, tableName, columnName).Scan(&count); err != nil {
		t.Fatalf("query information_schema.columns for %q.%q: %v", tableName, columnName, err)
	}
	return count == 1
}

// insertUserPrefAuthUser creates a row in auth.users and returns its UUID.
// The on-insert trigger creates the matching public.users row automatically.
func insertUserPrefAuthUser(t *testing.T, ctx context.Context, sqlDB *sql.DB) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO auth.users (id, email) VALUES ($1, $2)`,
		id, fmt.Sprintf("%s@preftest", id)); err != nil {
		t.Fatalf("insert auth.users: %v", err)
	}
	return id
}

// insertCardgroupForPref inserts a minimal cardgroup row and returns its UUID.
func insertCardgroupForPref(t *testing.T, ctx context.Context, sqlDB *sql.DB, ownerID string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.cardgroups (id, owner_id, name) VALUES ($1, $2, $3)`,
		id, ownerID, fmt.Sprintf("pref-cg-%s", id)); err != nil {
		t.Fatalf("insert cardgroup: %v", err)
	}
	return id
}

// queryNullableUUID scans a single nullable UUID column. Returns ("", false)
// when the value is NULL.
func queryNullableUUID(t *testing.T, ctx context.Context, sqlDB *sql.DB, query string, args ...any) (string, bool) {
	t.Helper()
	var val sql.NullString
	if err := sqlDB.QueryRowContext(ctx, query, args...).Scan(&val); err != nil {
		t.Fatalf("query nullable uuid: %v", err)
	}
	return val.String, val.Valid
}

// rlsPolicyCount returns how many RLS policies are attached to the given table.
func rlsPolicyCount(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName string) int {
	t.Helper()
	var count int
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pg_policies
		WHERE schemaname = 'public'
		  AND tablename  = $1
	`, tableName).Scan(&count); err != nil {
		t.Fatalf("query pg_policies for %q: %v", tableName, err)
	}
	return count
}

// TestExtractUserPreferences_DownUpRoundtrip exercises both directions of the
// 20260516120000_extract_user_preferences migration:
//
//  1. Starting point: fully migrated-up state — user_preferences exists,
//     users.last_viewed_cardgroup_id does not.
//  2. Seed user A (with a preference row) and user B (without one).
//  3. Down migration: verify last_viewed_cardgroup_id is re-added to users,
//     user A's value is backfilled, user B stays NULL, user_preferences is dropped.
//  4. Up migration (re-apply): verify user_preferences is re-created, user A's
//     row is backfilled, last_viewed_cardgroup_id is removed from users, and
//     RLS policies are re-applied.
func TestExtractUserPreferences_DownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)

	// Sanity: the fully-migrated schema must have user_preferences and must NOT
	// have users.last_viewed_cardgroup_id.
	if !tableExists(t, ctx, sqlDB, "user_preferences") {
		t.Fatal("precondition: user_preferences table must exist after migrate up")
	}
	if columnExists(t, ctx, sqlDB, "users", "last_viewed_cardgroup_id") {
		t.Fatal("precondition: users.last_viewed_cardgroup_id must be absent after migrate up")
	}

	// --- seed test data (fully-migrated schema) ---

	userA := insertUserPrefAuthUser(t, ctx, sqlDB)
	userB := insertUserPrefAuthUser(t, ctx, sqlDB)
	cardgroupC := insertCardgroupForPref(t, ctx, sqlDB, userA)

	// User A has a preference row pointing at cardgroupC.
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_preferences (user_id, last_viewed_cardgroup_id)
		 VALUES ($1, $2)`,
		userA, cardgroupC); err != nil {
		t.Fatalf("insert user_preferences for user A: %v", err)
	}
	// User B intentionally has no user_preferences row.

	// --- Down migration ---

	if err := database.MigrateStepsForTest(testDSN, -1); err != nil {
		t.Fatalf("migrate down 1 step: %v", err)
	}

	// 3a. users.last_viewed_cardgroup_id must have been re-added.
	if !columnExists(t, ctx, sqlDB, "users", "last_viewed_cardgroup_id") {
		t.Fatal("after down: users.last_viewed_cardgroup_id must exist")
	}

	// 3b. user A's preference is backfilled onto the users column.
	gotA, okA := queryNullableUUID(t, ctx, sqlDB,
		`SELECT last_viewed_cardgroup_id FROM public.users WHERE id = $1`, userA)
	if !okA {
		t.Fatal("after down: users.last_viewed_cardgroup_id for user A must be non-NULL")
	}
	if gotA != cardgroupC {
		t.Fatalf("after down: user A last_viewed_cardgroup_id = %q, want %q", gotA, cardgroupC)
	}

	// 3c. user B has no preference row, so the column must stay NULL.
	_, okB := queryNullableUUID(t, ctx, sqlDB,
		`SELECT last_viewed_cardgroup_id FROM public.users WHERE id = $1`, userB)
	if okB {
		t.Fatal("after down: users.last_viewed_cardgroup_id for user B must be NULL")
	}

	// 3d. user_preferences table must have been dropped.
	if tableExists(t, ctx, sqlDB, "user_preferences") {
		t.Fatal("after down: user_preferences table must not exist")
	}

	// --- Up migration (re-apply) ---

	if err := database.MigrateStepsForTest(testDSN, 1); err != nil {
		t.Fatalf("migrate up 1 step: %v", err)
	}

	// 4a. user_preferences table must be re-created.
	if !tableExists(t, ctx, sqlDB, "user_preferences") {
		t.Fatal("after re-up: user_preferences table must exist")
	}

	// 4b. user A's preference is backfilled from users into user_preferences.
	gotAUp, okAUp := queryNullableUUID(t, ctx, sqlDB,
		`SELECT last_viewed_cardgroup_id FROM public.user_preferences WHERE user_id = $1`, userA)
	if !okAUp {
		t.Fatal("after re-up: user_preferences.last_viewed_cardgroup_id for user A must be non-NULL")
	}
	if gotAUp != cardgroupC {
		t.Fatalf("after re-up: user A user_preferences.last_viewed_cardgroup_id = %q, want %q", gotAUp, cardgroupC)
	}

	// 4c. users.last_viewed_cardgroup_id must be gone again.
	if columnExists(t, ctx, sqlDB, "users", "last_viewed_cardgroup_id") {
		t.Fatal("after re-up: users.last_viewed_cardgroup_id must have been dropped")
	}

	// 4d. RLS policies on user_preferences must be re-applied (4 policies: select,
	// insert, update, delete).
	if n := rlsPolicyCount(t, ctx, sqlDB, "user_preferences"); n != 4 {
		t.Fatalf("after re-up: expected 4 RLS policies on user_preferences, got %d", n)
	}
}

// TestExtractUserPreferences_NullPrefHandledByDown verifies that a
// user_preferences row whose last_viewed_cardgroup_id is NULL (e.g. after an
// ON DELETE SET NULL cascade) is handled safely by the down migration: the
// UPDATE in the down script skips rows where p.last_viewed_cardgroup_id IS NULL,
// leaving the users column NULL rather than overwriting an existing non-NULL
// value with NULL.
func TestExtractUserPreferences_NullPrefHandledByDown(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)

	if !tableExists(t, ctx, sqlDB, "user_preferences") {
		t.Fatal("precondition: user_preferences table must exist")
	}

	// Create user C with a preference row whose last_viewed_cardgroup_id is NULL.
	userC := insertUserPrefAuthUser(t, ctx, sqlDB)
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_preferences (user_id, last_viewed_cardgroup_id)
		 VALUES ($1, NULL)`,
		userC); err != nil {
		t.Fatalf("insert null-pref row for user C: %v", err)
	}

	// Down migration must succeed without error.
	if err := database.MigrateStepsForTest(testDSN, -1); err != nil {
		t.Fatalf("migrate down 1 step: %v", err)
	}

	// user C's last_viewed_cardgroup_id must remain NULL on users.
	_, okC := queryNullableUUID(t, ctx, sqlDB,
		`SELECT last_viewed_cardgroup_id FROM public.users WHERE id = $1`, userC)
	if okC {
		t.Fatal("after down: user C last_viewed_cardgroup_id must be NULL (null pref must not overwrite)")
	}

	// Re-apply so the DB ends in the fully-migrated-up state for the rest of the suite.
	if err := database.MigrateStepsForTest(testDSN, 1); err != nil {
		t.Fatalf("migrate up 1 step (restore): %v", err)
	}

	if !tableExists(t, ctx, sqlDB, "user_preferences") {
		t.Fatal("after restore: user_preferences table must exist")
	}
}
