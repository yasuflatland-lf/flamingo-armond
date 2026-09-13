package database_test

import (
	"context"
	"database/sql"
	"testing"

	"backend/internal/database"
)

// TestNewCardRatioDefaultDownUpRoundtrip pins the new_card_ratio_num column
// default against a real Postgres: 1/5 at head, 4/5 after rolling back
// lower_new_card_ratio_default, 1/5 again after reapplying it. It only checks
// the default applied to freshly inserted rows; it does not assert that
// pre-existing rows survive the roundtrip.
func TestNewCardRatioDefaultDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireDefaultRatio(t, ctx, sqlDB, 1, 5)

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
		t.Fatalf("migrate down lower_new_card_ratio_default: %v", err)
	}
	requireDefaultRatio(t, ctx, sqlDB, 4, 5)

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up lower_new_card_ratio_default: %v", err)
	}
	requireDefaultRatio(t, ctx, sqlDB, 1, 5)
}

func requireDefaultRatio(t *testing.T, ctx context.Context, sqlDB *sql.DB, wantNum, wantDen int) {
	t.Helper()
	userID := insertRLSAuthUser(t, ctx, sqlDB)

	var gotNum, gotDen int
	if err := sqlDB.QueryRowContext(ctx, `
		INSERT INTO public.user_preferences (user_id)
		VALUES ($1)
		RETURNING new_card_ratio_num, new_card_ratio_den
	`, userID).Scan(&gotNum, &gotDen); err != nil {
		t.Fatalf("insert user_preferences with defaults: %v", err)
	}
	if gotNum != wantNum || gotDen != wantDen {
		t.Fatalf("default ratio: got %d/%d, want %d/%d", gotNum, gotDen, wantNum, wantDen)
	}
}
