package database_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"backend/internal/database"
)

func TestUsersVersionDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireColumnExists(t, ctx, sqlDB, "users", "version")

	userID := insertRoundtripUser(t, ctx, sqlDB)
	assertUserProfile(t, ctx, sqlDB, userID, "Roundtrip User", "bio before rollback", "https://example.com/avatar.png")
	assertUserVersion(t, ctx, sqlDB, userID, 0)

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Step back through the 18 migrations listed newest-first until
	// add_version_to_users (the target) is also rolled back:
	//   1. widen_updated_at_triggers_to_insert
	//   2. widen_text_length_checks
	//   3. add_cardgroup_fk_to_swipe_records
	//   4. add_stability_before_to_swipe_records
	//   5. add_last_rating_to_user_card_fsrs
	//   6. add_pre_swipe_snapshot_to_swipe_records
	//   7. add_new_card_ratio_to_user_preferences
	//   8. index_hygiene_users_swipe_records
	//   9. drop_master_cardgroup_metadata_columns
	//   10. master_cards_front_citext
	//   11. add_user_card_fsrs_card_id_index
	//   12. add_learn_display_mode_to_user_preferences
	//   13. add_master_tables
	//   14. restrict_definer_function_exposure
	//   15. pin_trigger_function_search_path
	//   16. enable_rls_schema_migrations
	//   17. add_position_to_cards
	//   18. add_version_to_users  ← target (rolls back the version column)
	// Bump the count here when adding migrations after add_version_to_users.
	if err := m.Steps(-18); err != nil {
		t.Fatalf("migrate down to before add_version_to_users: %v", err)
	}

	requireColumnMissing(t, ctx, sqlDB, "users", "version")
	assertUserProfile(t, ctx, sqlDB, userID, "Roundtrip User", "bio before rollback", "https://example.com/avatar.png")

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up one step: %v", err)
	}

	requireColumnExists(t, ctx, sqlDB, "users", "version")
	assertUserProfile(t, ctx, sqlDB, userID, "Roundtrip User", "bio before rollback", "https://example.com/avatar.png")
	assertUserVersion(t, ctx, sqlDB, userID, 0)
}

func insertRoundtripUser(t *testing.T, ctx context.Context, sqlDB *sql.DB) string {
	t.Helper()

	userID := uuid.NewString()
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO auth.users (id, email) VALUES ($1, $2)`,
		userID, fmt.Sprintf("%s@test", userID)); err != nil {
		t.Fatalf("insert auth.users: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE public.users
		 SET display_name = $2,
		     bio = $3,
		     avatar_url = $4
		 WHERE id = $1`,
		userID, "Roundtrip User", "bio before rollback", "https://example.com/avatar.png"); err != nil {
		t.Fatalf("seed public.users: %v", err)
	}
	return userID
}

func requireColumnExists(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName, columnName string) {
	t.Helper()
	if !columnExists(t, ctx, sqlDB, tableName, columnName) {
		t.Fatalf("expected column %s.%s to exist", tableName, columnName)
	}
}

func requireColumnMissing(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName, columnName string) {
	t.Helper()
	if columnExists(t, ctx, sqlDB, tableName, columnName) {
		t.Fatalf("expected column %s.%s to be absent", tableName, columnName)
	}
}

func columnExists(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName, columnName string) bool {
	t.Helper()

	var count int
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT count(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = $1
		  AND column_name = $2
	`, tableName, columnName).Scan(&count); err != nil {
		t.Fatalf("query column %s.%s: %v", tableName, columnName, err)
	}
	return count == 1
}

func assertUserProfile(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID, wantDisplayName, wantBio, wantAvatarURL string) {
	t.Helper()

	var displayName, bio, avatarURL sql.NullString
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT display_name, bio, avatar_url
		FROM public.users
		WHERE id = $1
	`, userID).Scan(&displayName, &bio, &avatarURL); err != nil {
		t.Fatalf("query public.users row: %v", err)
	}

	if !displayName.Valid || displayName.String != wantDisplayName {
		t.Fatalf("display_name: got %v, want %q", displayName, wantDisplayName)
	}
	if !bio.Valid || bio.String != wantBio {
		t.Fatalf("bio: got %v, want %q", bio, wantBio)
	}
	if !avatarURL.Valid || avatarURL.String != wantAvatarURL {
		t.Fatalf("avatar_url: got %v, want %q", avatarURL, wantAvatarURL)
	}
}

func assertUserVersion(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID string, want int64) {
	t.Helper()

	var got int64
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT version
		FROM public.users
		WHERE id = $1
	`, userID).Scan(&got); err != nil {
		t.Fatalf("query public.users.version: %v", err)
	}
	if got != want {
		t.Fatalf("version: got %d, want %d", got, want)
	}
}
