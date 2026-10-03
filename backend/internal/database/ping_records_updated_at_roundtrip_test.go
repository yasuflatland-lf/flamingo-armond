package database_test

import (
	"context"
	"database/sql"
	"testing"

	"backend/internal/database"
)

// TestDropPingRecordsUpdatedAt_DownUpRoundtrip pins the rollback shape: the
// down restores ping_records.updated_at as timestamptz NOT NULL DEFAULT now()
// on a table that already holds a row, and the up drops it again.
//
// t.Parallel() is intentionally absent: migrations change global suite state.
func TestDropPingRecordsUpdatedAt_DownUpRoundtrip(t *testing.T) {
	// Named versions avoid relative step counts that drift after new migrations.
	const (
		beforeDrop uint = 20260927000006
		afterDrop  uint = 20260927000007
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
	requireColumnMissing(t, ctx, sqlDB, "ping_records", "updated_at")

	var pingID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO public.ping_records DEFAULT VALUES RETURNING id`).Scan(&pingID); err != nil {
		t.Fatalf("seed ping_records: %v", err)
	}
	defer func() {
		if _, err := sqlDB.ExecContext(ctx, `DELETE FROM public.ping_records WHERE id = $1`, pingID); err != nil {
			t.Errorf("delete seeded ping_records row: %v", err)
		}
	}()

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
	requireColumnExists(t, ctx, sqlDB, "ping_records", "updated_at")
	requireColumnNotNull(t, ctx, sqlDB, "ping_records", "updated_at")
	requireRestoredPingRecordsUpdatedAtShape(t, ctx, sqlDB)
	requirePingRecordRowCount(t, ctx, sqlDB, pingID, 1)

	if err := m.Migrate(afterDrop); err != nil {
		t.Fatalf("migrate to version %d: %v", afterDrop, err)
	}
	requireColumnMissing(t, ctx, sqlDB, "ping_records", "updated_at")
	requirePingRecordRowCount(t, ctx, sqlDB, pingID, 1)
}

func requireRestoredPingRecordsUpdatedAtShape(t *testing.T, ctx context.Context, sqlDB *sql.DB) {
	t.Helper()
	var dataType string
	var columnDefault sql.NullString
	if err := sqlDB.QueryRowContext(ctx, `
		SELECT data_type, column_default
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'ping_records'
		  AND column_name = 'updated_at'
	`).Scan(&dataType, &columnDefault); err != nil {
		t.Fatalf("query ping_records.updated_at shape: %v", err)
	}
	if dataType != "timestamp with time zone" {
		t.Fatalf("ping_records.updated_at data_type = %q, want %q", dataType, "timestamp with time zone")
	}
	if !columnDefault.Valid || columnDefault.String != "now()" {
		t.Fatalf("ping_records.updated_at column_default = %v, want %q", columnDefault, "now()")
	}
}

func requirePingRecordRowCount(t *testing.T, ctx context.Context, sqlDB *sql.DB, pingID string, want int) {
	t.Helper()
	var got int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM public.ping_records WHERE id = $1`, pingID).Scan(&got); err != nil {
		t.Fatalf("count ping_records row %s: %v", pingID, err)
	}
	if got != want {
		t.Fatalf("ping_records row %s count = %d, want %d", pingID, got, want)
	}
}
