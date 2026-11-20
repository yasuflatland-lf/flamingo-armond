package database_test

import (
	"context"
	"slices"
	"testing"

	"backend/internal/database"
)

// TestDropSwipeRecordsUserCardgroupIndex_DownUpRoundtrip pins the index shapes
// before and after rolling back the drop migration.
//
// t.Parallel() is intentionally absent: migrations change global suite state.
func TestDropSwipeRecordsUserCardgroupIndex_DownUpRoundtrip(t *testing.T) {
	// Named versions avoid relative step counts that drift after new migrations.
	const (
		beforeDrop uint = 20260927000002
		afterDrop  uint = 20260927000003
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
	wantDropped := []string{"USING btree (cardgroup_id)"}
	if got := swipeRecordsIndexShapesOnColumn(t, ctx, sqlDB, "cardgroup_id"); !slices.Equal(got, wantDropped) {
		t.Fatalf("index shapes at HEAD=%q, want %q", got, wantDropped)
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

	if err := m.Migrate(beforeDrop); err != nil {
		t.Fatalf("migrate to version %d: %v", beforeDrop, err)
	}
	wantRestored := []string{
		"USING btree (cardgroup_id)",
		"USING btree (user_id, cardgroup_id, reviewed_at DESC)",
	}
	if got := swipeRecordsIndexShapesOnColumn(t, ctx, sqlDB, "cardgroup_id"); !slices.Equal(got, wantRestored) {
		t.Fatalf("index shapes at version %d=%q, want %q", beforeDrop, got, wantRestored)
	}

	if err := m.Migrate(afterDrop); err != nil {
		t.Fatalf("migrate to version %d: %v", afterDrop, err)
	}
	if got := swipeRecordsIndexShapesOnColumn(t, ctx, sqlDB, "cardgroup_id"); !slices.Equal(got, wantDropped) {
		t.Fatalf("index shapes at version %d=%q, want %q", afterDrop, got, wantDropped)
	}
}
