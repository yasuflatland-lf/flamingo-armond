package database_test

import (
	"context"
	"database/sql"
	"testing"

	"backend/internal/database"
)

// TestDropUserCardFSRSLastRating_DownUpRoundtrip pins the rollback shape: the
// down restores last_rating as a nullable smallint without re-running the
// swipe_records backfill, which the pre-drop mapper needs to read rows.
//
// t.Parallel() is intentionally absent: migrations change global suite state.
func TestDropUserCardFSRSLastRating_DownUpRoundtrip(t *testing.T) {
	// Named versions avoid relative step counts that drift after new migrations.
	const (
		beforeDrop uint = 20260927000003
		afterDrop  uint = 20260927000004
	)

	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireColumnMissing(t, ctx, sqlDB, "user_card_fsrs", "last_rating")

	userID := insertRLSAuthUser(t, ctx, sqlDB)
	cardgroupID := insertRLSCardgroup(t, ctx, sqlDB, userID, "Last rating drop")
	cardID := insertRLSCard(t, ctx, sqlDB, cardgroupID, "Last rating drop card")
	insertRLSUserCardFSRS(t, ctx, sqlDB, userID, cardID)
	insertRLSSwipe(t, ctx, sqlDB, userID, cardID, cardgroupID)

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	if err := m.Migrate(beforeDrop); err != nil {
		t.Fatalf("migrate to version %d: %v", beforeDrop, err)
	}
	requireColumnExists(t, ctx, sqlDB, "user_card_fsrs", "last_rating")
	requireColumnNullable(t, ctx, sqlDB, "user_card_fsrs", "last_rating")
	requireRestoredLastRatingShape(t, ctx, sqlDB)
	var lastRating sql.NullInt16
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT last_rating
		FROM public.user_card_fsrs
		WHERE user_id = $1 AND card_id = $2
	`, userID, cardID).Scan(&lastRating); err != nil {
		t.Fatalf("read restored last_rating: %v", err)
	}
	if lastRating.Valid {
		t.Fatalf("restored last_rating = %d, want NULL; down must not re-run the backfill", lastRating.Int16)
	}
	requireUserCardFSRSRowCount(t, ctx, sqlDB, userID, cardID, 1)

	if err := m.Migrate(afterDrop); err != nil {
		t.Fatalf("migrate to version %d: %v", afterDrop, err)
	}
	requireColumnMissing(t, ctx, sqlDB, "user_card_fsrs", "last_rating")
	requireUserCardFSRSRowCount(t, ctx, sqlDB, userID, cardID, 1)
}

func requireRestoredLastRatingShape(t *testing.T, ctx context.Context, sqlDB *sql.DB) {
	t.Helper()
	var dataType string
	var columnDefault sql.NullString
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT data_type, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'user_card_fsrs'
		  AND column_name = 'last_rating'
	`).Scan(&dataType, &columnDefault); err != nil {
		t.Fatalf("query user_card_fsrs.last_rating shape: %v", err)
	}
	if dataType != "smallint" {
		t.Fatalf("user_card_fsrs.last_rating data_type = %q, want %q", dataType, "smallint")
	}
	if columnDefault.Valid {
		t.Fatalf("user_card_fsrs.last_rating column_default = %q, want NULL", columnDefault.String)
	}
}
