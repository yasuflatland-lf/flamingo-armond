package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"backend/internal/database"
)

// TestLearnDisplayModeDownUpRoundtrip proves that the down migration for
// 20260615000000_add_learn_display_mode_to_user_preferences correctly drops
// the column and that the up migration re-adds it, while leaving existing
// user_preferences data intact.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestLearnDisplayModeDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireColumnExists(t, ctx, sqlDB, "user_preferences", "learn_display_mode")

	userID := insertLearnDisplayModeUser(t, ctx, sqlDB)

	// Assert the column default is 'flip_to_reveal'.
	assertLearnDisplayMode(t, ctx, sqlDB, userID, "flip_to_reveal")

	// Update to 'always_visible' and assert it persists.
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE public.user_preferences SET learn_display_mode = 'always_visible' WHERE user_id = $1`,
		userID); err != nil {
		t.Fatalf("update learn_display_mode: %v", err)
	}
	assertLearnDisplayMode(t, ctx, sqlDB, userID, "always_visible")

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Step back eleven migrations newest-first:
	// widen_text_length_checks (now the newest),
	// add_cardgroup_fk_to_swipe_records,
	// add_stability_before_to_swipe_records,
	// add_last_rating_to_user_card_fsrs, add_pre_swipe_snapshot_to_swipe_records,
	// add_new_card_ratio_to_user_preferences,
	// index_hygiene_users_swipe_records,
	// drop_master_cardgroup_metadata_columns, master_cards_front_citext,
	// add_user_card_fsrs_card_id_index,
	// then add_learn_display_mode_to_user_preferences (the target). The Steps(1)
	// below re-applies add_learn_display_mode, and the t.Cleanup restores the
	// rest. Bump this count when adding migrations after
	// add_learn_display_mode_to_user_preferences.
	if err := m.Steps(-12); err != nil {
		t.Fatalf("migrate down to before add_learn_display_mode_to_user_preferences: %v", err)
	}

	requireColumnMissing(t, ctx, sqlDB, "user_preferences", "learn_display_mode")

	// The row itself must still be present after the column is dropped.
	var count int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM public.user_preferences WHERE user_id = $1`, userID,
	).Scan(&count); err != nil {
		t.Fatalf("query user_preferences after down: %v", err)
	}
	if count != 1 {
		t.Fatalf("user_preferences row gone after down migration: got %d rows", count)
	}

	// Re-apply the up migration.
	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up one step: %v", err)
	}

	requireColumnExists(t, ctx, sqlDB, "user_preferences", "learn_display_mode")

	// After re-applying up, the column must be back and default to 'flip_to_reveal'
	// for the surviving row (the previously set 'always_visible' value is gone
	// because the column was dropped and re-added with a NOT NULL DEFAULT).
	assertLearnDisplayMode(t, ctx, sqlDB, userID, "flip_to_reveal")
}

// insertLearnDisplayModeUser inserts rows in auth.users, public.users (via
// trigger), and public.user_preferences to satisfy FK constraints, then
// returns the user ID.
func insertLearnDisplayModeUser(t *testing.T, ctx context.Context, sqlDB *sql.DB) string {
	t.Helper()

	userID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO auth.users (id, email) VALUES ($1, $2)`,
		userID, fmt.Sprintf("%s@test", userID)); err != nil {
		t.Fatalf("insert auth.users: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO public.user_preferences (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`,
		userID); err != nil {
		t.Fatalf("insert public.user_preferences: %v", err)
	}
	return userID
}

func assertLearnDisplayMode(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID, want string) {
	t.Helper()

	var got string
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT learn_display_mode FROM public.user_preferences WHERE user_id = $1`,
		userID,
	).Scan(&got); err != nil {
		t.Fatalf("query public.user_preferences.learn_display_mode: %v", err)
	}
	if got != want {
		t.Fatalf("learn_display_mode: got %q, want %q", got, want)
	}
}
