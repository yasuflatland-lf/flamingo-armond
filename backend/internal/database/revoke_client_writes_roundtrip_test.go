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
	requireInsertPrivilege := func(want bool) {
		t.Helper()
		var granted bool
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT has_table_privilege('authenticated', 'public.cards', 'INSERT')`).Scan(&granted); err != nil {
			t.Fatalf("query authenticated cards INSERT privilege: %v", err)
		}
		if granted != want {
			t.Fatalf("authenticated cards INSERT privilege: got %t, want %t", granted, want)
		}
	}
	requireInsertPrivilege(false)

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
		t.Fatalf("migrate down revoke_client_writes: %v", err)
	}
	requireInsertPrivilege(true)
	if err := m.Steps(1); err != nil {
		t.Fatalf("migrate up revoke_client_writes: %v", err)
	}
	requireInsertPrivilege(false)
}
