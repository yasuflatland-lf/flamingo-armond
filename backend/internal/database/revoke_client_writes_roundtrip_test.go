package database_test

import (
	"context"
	"testing"

	"backend/internal/database"
)

func TestRevokeClientWritesDownUpRoundtrip(t *testing.T) {
	ctx := context.Background()
	db := openMigratedDB(t)
	defer db.Close()
	t.Cleanup(func() {
		if err := database.Migrate(testDSN); err != nil {
			t.Errorf("restore latest migration: %v", err)
		}
	})
	sqlDB := sqlDBForTest(t, db)
	requireAPIWritePrivileges(t, ctx, sqlDB, false)
	requireDefaultAPIWritePrivileges(t, ctx, sqlDB, false)

	m, err := database.NewMigrateInstanceForTest(testDSN)
	if err != nil {
		t.Fatalf("NewMigrateInstanceForTest: %v", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			t.Logf("migrate close: src_err=%v db_err=%v", srcErr, dbErr)
		}
	}()
	// drop_ping_records_updated_at, cardgroups_owner_updated_at_index,
	// drop_redundant_fk_indexes, drop_user_card_fsrs_last_rating,
	// drop_swipe_records_user_cardgroup_index and reset_legacy_new_card_ratio sit
	// above revoke_client_writes, so seven steps reach it.
	if err := m.Steps(-7); err != nil {
		t.Fatalf("migrate down revoke_client_writes: %v", err)
	}
	requireAPIWritePrivileges(t, ctx, sqlDB, true)
	requireDefaultAPIWritePrivileges(t, ctx, sqlDB, true)
	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up revoke_client_writes: %v", err)
	}
	requireAPIWritePrivileges(t, ctx, sqlDB, false)
	requireDefaultAPIWritePrivileges(t, ctx, sqlDB, false)
}
