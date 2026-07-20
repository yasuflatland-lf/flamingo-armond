package database_test

import (
	"context"
	"testing"

	"backend/internal/database"
)

// TestStabilityBeforeDownUpRoundtrip proves that the newest migration drops and
// restores swipe_records.stability_before without leaving the schema behind the
// latest migration version.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestStabilityBeforeDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "stability_before")

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()

	// Step back three migrations newest-first: widen_text_length_checks,
	// add_cardgroup_fk_to_swipe_records, then add_stability_before_to_swipe_records
	// (the target). The Steps(1) below re-applies only stability_before; the
	// t.Cleanup restores the rest. Bump this count when adding migrations after
	// add_stability_before_to_swipe_records.
	if err := m.Steps(-3); err != nil {
		t.Fatalf("migrate down stability_before migration: %v", err)
	}
	requireColumnMissing(t, ctx, sqlDB, "swipe_records", "stability_before")
	// The sibling snapshot columns from the earlier migration must survive.
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "phase_before")
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "scheduled_days_before")

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up stability_before migration: %v", err)
	}
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "stability_before")
}
