package database_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"backend/internal/database"
)

// TestLastRatingDownUpRoundtrip proves that the add_last_rating migration drops
// and restores user_card_fsrs.last_rating without leaving the schema behind the
// latest migration version.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestLastRatingDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireColumnExists(t, ctx, sqlDB, "user_card_fsrs", "last_rating")

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Step back four migrations newest-first: widen_text_length_checks,
	// add_cardgroup_fk_to_swipe_records, add_stability_before_to_swipe_records,
	// then add_last_rating_to_user_card_fsrs (the target). Bump this count when
	// adding migrations after add_last_rating_to_user_card_fsrs.
	if err := m.Steps(-4); err != nil {
		t.Fatalf("migrate down to before last_rating migration: %v", err)
	}
	requireColumnMissing(t, ctx, sqlDB, "user_card_fsrs", "last_rating")

	if err := m.Steps(4); err != nil {
		t.Fatalf("migrate up last_rating migration: %v", err)
	}
	requireColumnExists(t, ctx, sqlDB, "user_card_fsrs", "last_rating")
}

// TestLastRatingUpMigrationBackfillsLatestSwipe verifies that the up migration
// restores the rating from the latest swipe and leaves rows without swipe
// history NULL. The swipe IDs sort opposite to their reviewed_at timestamps so
// the test also pins reviewed_at as the primary latest-record key.
func TestLastRatingUpMigrationBackfillsLatestSwipe(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Four steps: widen_text_length_checks, add_cardgroup_fk_to_swipe_records and
	// add_stability_before_to_swipe_records sit above the target.
	if err := m.Steps(-4); err != nil {
		t.Fatalf("migrate down to before last_rating migration: %v", err)
	}
	requireColumnMissing(t, ctx, sqlDB, "user_card_fsrs", "last_rating")

	userID := insertAuthUserForAdmin(t, ctx, db)
	cardgroupID := insertRLSCardgroup(t, ctx, sqlDB, userID, "Last Rating Backfill")
	withHistoryID := insertRLSCard(t, ctx, sqlDB, cardgroupID, "with swipe history")
	withoutHistoryID := insertRLSCard(t, ctx, sqlDB, cardgroupID, "without swipe history")
	olderReviewedAt := time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC)
	newerReviewedAt := olderReviewedAt.Add(time.Hour)

	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO public.user_card_fsrs (
			user_id, card_id, state, due, stability, difficulty,
			reps, lapses, last_review, elapsed_days, scheduled_days
		)
		VALUES
			($1, $2, 2, $4, 6.9, 5.0, 2, 0, $4, 1, 1),
			($1, $3, 2, $4, 6.9, 5.0, 1, 0, $4, 1, 1)
	`, userID, withHistoryID, withoutHistoryID, newerReviewedAt); err != nil {
		t.Fatalf("seed user_card_fsrs rows: %v", err)
	}

	const olderSwipeID = "ffffffff-ffff-4fff-bfff-ffffffffffff"
	const newerSwipeID = "00000000-0000-4000-8000-000000000001"
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO public.swipe_records (
			id, user_id, card_id, cardgroup_id, rating, reviewed_at,
			due, stability, difficulty, elapsed_days, scheduled_days,
			reps, lapses, state, last_review
		)
		VALUES
			($1, $3, $4, $5, 2, $6, $6, 5.0, 5.0, 1, 1, 1, 0, 2, $6),
			($2, $3, $4, $5, 4, $7, $7, 6.9, 5.0, 1, 1, 2, 0, 2, $7)
	`, olderSwipeID, newerSwipeID, userID, withHistoryID, cardgroupID, olderReviewedAt, newerReviewedAt); err != nil {
		t.Fatalf("seed swipe_records rows: %v", err)
	}

	if err := m.Steps(4); err != nil {
		t.Fatalf("migrate up last_rating migration: %v", err)
	}
	requireColumnExists(t, ctx, sqlDB, "user_card_fsrs", "last_rating")

	assertLastRating(t, ctx, sqlDB, userID, withHistoryID, sql.NullInt64{Int64: 4, Valid: true})
	assertLastRating(t, ctx, sqlDB, userID, withoutHistoryID, sql.NullInt64{})
}

func assertLastRating(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID, cardID string, want sql.NullInt64) {
	t.Helper()
	var got sql.NullInt64
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT last_rating FROM public.user_card_fsrs WHERE user_id = $1 AND card_id = $2`,
		userID, cardID).Scan(&got); err != nil {
		t.Fatalf("query user_card_fsrs.last_rating: %v", err)
	}
	if got != want {
		t.Fatalf("last_rating for card %s: got %v, want %v", cardID, got, want)
	}
}
