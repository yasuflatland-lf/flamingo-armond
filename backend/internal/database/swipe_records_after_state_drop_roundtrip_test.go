package database_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"

	"backend/internal/database"
)

// swipeRecordsAfterStateColumns maps each post-swipe column that
// 20260927000008_drop_swipe_records_after_state drops to its initial-schema type.
var swipeRecordsAfterStateColumns = map[string]string{
	"due":            "timestamp with time zone",
	"stability":      "double precision",
	"scheduled_days": "integer",
	"reps":           "integer",
	"lapses":         "integer",
	"state":          "integer",
	"last_review":    "timestamp with time zone",
}

// TestDropSwipeRecordsAfterState_DownUpRoundtrip pins the rollback shape: the
// down restores the seven columns as NOT NULL with no default even though a row
// already exists, and difficulty survives both directions.
//
// t.Parallel() is intentionally absent: migrations change global suite state.
func TestDropSwipeRecordsAfterState_DownUpRoundtrip(t *testing.T) {
	// Named versions avoid relative step counts that drift after new migrations.
	const (
		beforeDrop uint = 20260927000007
		afterDrop  uint = 20260927000008
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
	for column := range swipeRecordsAfterStateColumns {
		requireColumnMissing(t, ctx, sqlDB, "swipe_records", column)
	}
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "difficulty")

	userID := insertRLSAuthUser(t, ctx, sqlDB)
	cardgroupID := insertRLSCardgroup(t, ctx, sqlDB, userID, "After state drop")
	cardID := insertRLSCard(t, ctx, sqlDB, cardgroupID, "After state drop card")
	swipeID := uuid.NewString()
	insertSwipeRecordWithID(t, ctx, sqlDB, swipeID, userID, cardID, cardgroupID)

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
	for column, dataType := range swipeRecordsAfterStateColumns {
		requireColumnNotNull(t, ctx, sqlDB, "swipe_records", column)
		requireRestoredAfterStateColumnShape(t, ctx, sqlDB, column, dataType)
	}
	requireSwipeRecordRowCount(t, ctx, sqlDB, swipeID, 1)
	requireSwipeRecordDifficulty(t, ctx, sqlDB, swipeID, 5.0)

	if err := m.Migrate(afterDrop); err != nil {
		t.Fatalf("migrate to version %d: %v", afterDrop, err)
	}
	for column := range swipeRecordsAfterStateColumns {
		requireColumnMissing(t, ctx, sqlDB, "swipe_records", column)
	}
	requireSwipeRecordRowCount(t, ctx, sqlDB, swipeID, 1)
	requireSwipeRecordDifficulty(t, ctx, sqlDB, swipeID, 5.0)
}

func requireRestoredAfterStateColumnShape(t *testing.T, ctx context.Context, sqlDB *sql.DB, column, wantType string) {
	t.Helper()
	var dataType string
	var columnDefault sql.NullString
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT data_type, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'swipe_records'
		  AND column_name = $1
	`, column).Scan(&dataType, &columnDefault); err != nil {
		t.Fatalf("query swipe_records.%s shape: %v", column, err)
	}
	if dataType != wantType {
		t.Fatalf("swipe_records.%s data_type = %q, want %q", column, dataType, wantType)
	}
	if columnDefault.Valid {
		t.Fatalf("swipe_records.%s column_default = %q, want NULL", column, columnDefault.String)
	}
}

func requireSwipeRecordDifficulty(t *testing.T, ctx context.Context, sqlDB *sql.DB, swipeID string, want float64) {
	t.Helper()
	var got float64
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT difficulty FROM public.swipe_records WHERE id = $1`,
		swipeID).Scan(&got); err != nil {
		t.Fatalf("read swipe_records.difficulty: %v", err)
	}
	if got != want {
		t.Fatalf("swipe_records.difficulty = %v, want %v", got, want)
	}
}
