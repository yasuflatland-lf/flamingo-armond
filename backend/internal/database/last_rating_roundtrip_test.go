package database_test

import (
	"context"
	"testing"

	"backend/internal/database"
)

// TestLastRatingDownUpRoundtrip proves that the newest migration drops and
// restores user_card_fsrs.last_rating without leaving the schema behind the
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

	if err := m.Steps(-1); err != nil {
		t.Fatalf("migrate down last_rating migration: %v", err)
	}
	requireColumnMissing(t, ctx, sqlDB, "user_card_fsrs", "last_rating")

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up last_rating migration: %v", err)
	}
	requireColumnExists(t, ctx, sqlDB, "user_card_fsrs", "last_rating")
}
