package database_test

import (
	"context"
	"database/sql"
	"testing"

	"backend/internal/database"
)

// TestNewCardRatioCheckDownUpRoundtrip pins the tightened
// user_preferences_new_card_ratio_check against a real Postgres and proves the
// down migration restores the looser three-clause form.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestNewCardRatioCheckDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)

	// Tightened constraint in force.
	requireRatioAccepted(t, ctx, sqlDB, 4, 5)
	requireRatioAccepted(t, ctx, sqlDB, 3, 10)
	requireRatioRejected(t, ctx, sqlDB, 1, 3)   // 3 does not divide the 20-card session
	requireRatioRejected(t, ctx, sqlDB, 1, 21)  // reduced denominator above the cap
	requireRatioRejected(t, ctx, sqlDB, 19, 20) // new share above 4/5
	requireRatioRejected(t, ctx, sqlDB, 20, 21) // admitted by the previous loose CHECK

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// tighten_new_card_ratio_check is the newest migration, so one step down
	// restores the looser CHECK. Bump this count when adding later migrations.
	if err := m.Steps(-1); err != nil {
		t.Fatalf("migrate down tighten_new_card_ratio_check: %v", err)
	}
	requireRatioAccepted(t, ctx, sqlDB, 1, 3)
	requireRatioAccepted(t, ctx, sqlDB, 19, 20)
	requireRatioAccepted(t, ctx, sqlDB, 20, 21)
	requireRatioRejected(t, ctx, sqlDB, 1, 101)

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up tighten_new_card_ratio_check: %v", err)
	}
	requireRatioRejected(t, ctx, sqlDB, 1, 3)
	requireRatioRejected(t, ctx, sqlDB, 20, 21)
	requireRatioAccepted(t, ctx, sqlDB, 4, 5)
}

// insertRatioPref inserts a user_preferences row carrying num/den for a fresh
// user and returns the error the INSERT produced (nil on success). A new user
// per call keeps the PK-keyed 1:1 table free of ON CONFLICT interference.
func insertRatioPref(t *testing.T, ctx context.Context, sqlDB *sql.DB, num, den int) error {
	t.Helper()
	userID := insertRLSAuthUser(t, ctx, sqlDB)
	_, err := sqlDB.ExecContext(ctx, `
		INSERT INTO public.user_preferences (user_id, new_card_ratio_num, new_card_ratio_den)
		VALUES ($1, $2, $3)
	`, userID, num, den)
	return err
}

func requireRatioAccepted(t *testing.T, ctx context.Context, sqlDB *sql.DB, num, den int) {
	t.Helper()
	if err := insertRatioPref(t, ctx, sqlDB, num, den); err != nil {
		t.Fatalf("expected %d/%d to satisfy the new-card-ratio CHECK: %v", num, den, err)
	}
}

func requireRatioRejected(t *testing.T, ctx context.Context, sqlDB *sql.DB, num, den int) {
	t.Helper()
	if err := insertRatioPref(t, ctx, sqlDB, num, den); err == nil {
		t.Fatalf("expected %d/%d to violate the new-card-ratio CHECK, got no error", num, den)
	}
}
