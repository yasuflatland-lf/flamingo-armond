package database_test

import (
	"context"
	"testing"

	"backend/internal/database"
)

// TestSwipeRecordsCardgroupFKDownUpRoundtrip proves that the newest migration
// drops and restores both the swipe_records.cardgroup_id foreign key and the
// single-column index that backs it, without leaving the schema behind the latest
// migration version.
//
// t.Parallel() is intentionally absent: the test runs a global migration
// down/up that would race other tests in the package.
func TestSwipeRecordsCardgroupFKDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})

	sqlDB := sqlDBForTest(t, db)
	if _, ok := swipeRecordsCardgroupFKDeleteRule(t, ctx, sqlDB); !ok {
		t.Fatalf("precondition: foreign key %s must exist at HEAD", swipeRecordsCardgroupFKName)
	}
	if _, ok := swipeRecordsIndexDef(t, ctx, sqlDB, swipeRecordsCardgroupIndexName); !ok {
		t.Fatalf("precondition: index %s must exist at HEAD", swipeRecordsCardgroupIndexName)
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

	// Step back four migrations newest-first:
	// realign_fsrs_snapshot_columns_to_v4, widen_updated_at_triggers_to_insert,
	// widen_text_length_checks, then add_cardgroup_fk_to_swipe_records (the target).
	// The Steps(1) below
	// re-applies only the cardgroup FK; the t.Cleanup restores the rest. Bump
	// this count when adding migrations after add_cardgroup_fk_to_swipe_records.
	if err := m.Steps(-4); err != nil {
		t.Fatalf("migrate down cardgroup fk migration: %v", err)
	}
	if _, ok := swipeRecordsCardgroupFKDeleteRule(t, ctx, sqlDB); ok {
		t.Fatalf("foreign key %s survived the down migration", swipeRecordsCardgroupFKName)
	}
	if _, ok := swipeRecordsIndexDef(t, ctx, sqlDB, swipeRecordsCardgroupIndexName); ok {
		t.Fatalf("index %s survived the down migration", swipeRecordsCardgroupIndexName)
	}
	// The composite from the column-adding migration must survive: the down path
	// drops only what its own up path created.
	if _, ok := swipeRecordsIndexDef(t, ctx, sqlDB, "idx_swipe_records_user_cardgroup"); !ok {
		t.Fatal("idx_swipe_records_user_cardgroup was dropped by an unrelated down migration")
	}
	requireColumnExists(t, ctx, sqlDB, "swipe_records", "cardgroup_id")

	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up cardgroup fk migration: %v", err)
	}
	rule, ok := swipeRecordsCardgroupFKDeleteRule(t, ctx, sqlDB)
	if !ok {
		t.Fatalf("foreign key %s was not restored by the up migration", swipeRecordsCardgroupFKName)
	}
	if rule != "c" {
		t.Fatalf("restored %s confdeltype=%q, want %q (ON DELETE CASCADE)", swipeRecordsCardgroupFKName, rule, "c")
	}
	if _, ok := swipeRecordsIndexDef(t, ctx, sqlDB, swipeRecordsCardgroupIndexName); !ok {
		t.Fatalf("index %s was not restored by the up migration", swipeRecordsCardgroupIndexName)
	}
}
