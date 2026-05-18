package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-migrate/migrate/v4"

	"backend/internal/database"
)

// TestMigrateDownUpRoundtrip proves that the Down SQL is correct by running a
// full down→up cycle and asserting the expected public tables are present
// after the re-applied Up migration.
func TestMigrateDownUpRoundtrip(t *testing.T) {
	// Step 1: bring DB to post-up state.
	if err := database.Migrate(testDSN); err != nil {
		t.Fatalf("initial Migrate up: %v", err)
	}

	// Step 2: run Down to clean the DB.
	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate down: %v", err)
	}

	// Step 3: re-run Up to restore the schema.
	if err := database.Migrate(testDSN); err != nil {
		t.Fatalf("Migrate up after down: %v", err)
	}

	// Step 4: assert all expected tables exist in information_schema.
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()

	sqlDB := sqlDBForTest(t, db)
	want := []string{
		"users", "roles", "cardgroups", "cards",
		"user_card_fsrs", "user_preferences",
		"ping_records", "user_roles", "swipe_records",
	}
	for _, table := range want {
		var count int
		if err := sqlDB.QueryRowContext(ctx, `
			SELECT count(*)
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		`, table).Scan(&count); err != nil {
			t.Fatalf("query information_schema for %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("table %q not found in public schema after down→up roundtrip", table)
		}
	}
}
