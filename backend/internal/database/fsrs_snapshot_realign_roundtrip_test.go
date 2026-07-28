package database_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"backend/internal/database"
)

// TestFSRSSnapshotRealignDownUpRoundtrip proves that the migration restores the
// old shape without deleting data on the down path, then deliberately discards
// only swipe history when the up path is re-applied.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestFSRSSnapshotRealignDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "due_before")
	requireColumnMissing(t, ctx, sqlDB, "swipe_records", "elapsed_days")
	requireColumnMissing(t, ctx, sqlDB, "swipe_records", "scheduled_days_before")
	requireColumnMissing(t, ctx, sqlDB, "user_card_fsrs", "elapsed_days")
	requireColumnNotNull(t, ctx, sqlDB, "swipe_records", "phase_before")
	requireColumnNotNull(t, ctx, sqlDB, "swipe_records", "stability_before")
	requireColumnNotNull(t, ctx, sqlDB, "swipe_records", "due_before")

	userID := insertRLSAuthUser(t, ctx, sqlDB)
	cardgroupID := insertRLSCardgroup(t, ctx, sqlDB, userID, "FSRS snapshot realign")
	cardID := insertRLSCard(t, ctx, sqlDB, cardgroupID, "FSRS snapshot realign card")
	swipeID := uuid.NewString()
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)

	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO public.user_card_fsrs (
			user_id, card_id, state, due, stability, difficulty,
			reps, lapses, last_review, last_rating, scheduled_days
		)
		VALUES ($1, $2, 2, $3, 7.0, 5.0, 2, 0, $3, 3, 7)
	`, userID, cardID, now); err != nil {
		t.Fatalf("seed user_card_fsrs row: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, `
		INSERT INTO public.swipe_records (
			id, user_id, card_id, cardgroup_id, rating, reviewed_at,
			due, stability, difficulty, scheduled_days, reps, lapses, state,
			last_review, phase_before, stability_before, due_before
		)
		VALUES ($1, $2, $3, $4, 3, $5, $5, 8.0, 4.5, 8, 3, 0, 2,
			$5, 2, 7.0, $5)
	`, swipeID, userID, cardID, cardgroupID, now); err != nil {
		t.Fatalf("seed swipe_records row: %v", err)
	}

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	if err := m.Steps(-1); err != nil {
		t.Fatalf("migrate down FSRS snapshot realignment: %v", err)
	}
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "elapsed_days")
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "scheduled_days_before")
	requireColumnExists(t, ctx, sqlDB, "user_card_fsrs", "elapsed_days")
	requireColumnMissing(t, ctx, sqlDB, "swipe_records", "due_before")
	requireColumnNullable(t, ctx, sqlDB, "swipe_records", "phase_before")
	requireColumnNullable(t, ctx, sqlDB, "swipe_records", "stability_before")
	requireUserCardFSRSRowCount(t, ctx, sqlDB, userID, cardID, 1)
	requireSwipeRecordRowCount(t, ctx, sqlDB, swipeID, 1)

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up FSRS snapshot realignment: %v", err)
	}
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "due_before")
	requireColumnMissing(t, ctx, sqlDB, "swipe_records", "elapsed_days")
	requireColumnMissing(t, ctx, sqlDB, "swipe_records", "scheduled_days_before")
	requireColumnMissing(t, ctx, sqlDB, "user_card_fsrs", "elapsed_days")
	requireColumnNotNull(t, ctx, sqlDB, "swipe_records", "phase_before")
	requireColumnNotNull(t, ctx, sqlDB, "swipe_records", "stability_before")
	requireColumnNotNull(t, ctx, sqlDB, "swipe_records", "due_before")
	requireUserCardFSRSRowCount(t, ctx, sqlDB, userID, cardID, 1)
	requireSwipeRecordRowCount(t, ctx, sqlDB, swipeID, 0)
	requireTableRowCount(t, ctx, sqlDB, "swipe_records", 0)
}

func requireColumnNotNull(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName, columnName string) {
	t.Helper()
	requireColumnNullableState(t, ctx, sqlDB, tableName, columnName, "NO")
}

func requireColumnNullable(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName, columnName string) {
	t.Helper()
	requireColumnNullableState(t, ctx, sqlDB, tableName, columnName, "YES")
}

func requireColumnNullableState(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName, columnName, want string) {
	t.Helper()
	var got string
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = $1
		  AND column_name = $2
	`, tableName, columnName).Scan(&got); err != nil {
		t.Fatalf("query nullability for %s.%s: %v", tableName, columnName, err)
	}
	if got != want {
		t.Fatalf("%s.%s is_nullable = %q, want %q", tableName, columnName, got, want)
	}
}

func requireUserCardFSRSRowCount(t *testing.T, ctx context.Context, sqlDB *sql.DB, userID, cardID string, want int) {
	t.Helper()
	var got int
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT count(*)
		FROM public.user_card_fsrs
		WHERE user_id = $1 AND card_id = $2
	`, userID, cardID).Scan(&got); err != nil {
		t.Fatalf("count seeded user_card_fsrs row: %v", err)
	}
	if got != want {
		t.Fatalf("seeded user_card_fsrs row count = %d, want %d", got, want)
	}
}

func requireSwipeRecordRowCount(t *testing.T, ctx context.Context, sqlDB *sql.DB, swipeID string, want int) {
	t.Helper()
	var got int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM public.swipe_records WHERE id = $1`,
		swipeID).Scan(&got); err != nil {
		t.Fatalf("count seeded swipe_records row: %v", err)
	}
	if got != want {
		t.Fatalf("seeded swipe_records row count = %d, want %d", got, want)
	}
}

func requireTableRowCount(t *testing.T, ctx context.Context, sqlDB *sql.DB, tableName string, want int) {
	t.Helper()
	var got int
	if err := sqlDB.QueryRowContext(ctx, "SELECT count(*) FROM public."+tableName).Scan(&got); err != nil {
		t.Fatalf("count %s rows: %v", tableName, err)
	}
	if got != want {
		t.Fatalf("%s row count = %d, want %d", tableName, got, want)
	}
}
