package repository_test

// TestUserRoleRepository_AdminLockAndCountTx exercises the transaction-scoped
// pair the last-admin guard depends on. The advisory-lock statement is raw SQL,
// so only a real Postgres round-trip proves the call shape is valid; a typo in
// pg_advisory_xact_lock or hashtext would otherwise surface first in production.
//
// TestMain, testDB, insertAuthUser are defined in user_test.go and shared
// across this package. insertRole is defined in user_role_assign_test.go.

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"backend/internal/repository"
)

func TestUserRoleRepository_AdminLockAndCountTx(t *testing.T) {
	// Not parallel: the admin count is global, so a concurrent test assigning
	// the admin role would make the delta assertion flaky.
	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(testDB.GORM)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	role, err := roleRepo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	var before int64
	if terr := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if lerr := repo.AcquireAdminRoleLockTx(ctx, tx); lerr != nil {
			return lerr
		}
		var cerr error
		before, cerr = repo.CountAdminsTx(ctx, tx)
		return cerr
	}); terr != nil {
		t.Fatalf("lock + count (before assign): %v", terr)
	}
	if before < 0 {
		t.Errorf("CountAdminsTx: want >= 0, got %d", before)
	}

	userID := insertAuthUser(t, ctx)
	if err := repo.AssignRoleToUser(ctx, userID, role.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	// Re-taking the lock in a second transaction proves it was released at the
	// first transaction's commit rather than leaked for the session's lifetime.
	var after int64
	if terr := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if lerr := repo.AcquireAdminRoleLockTx(ctx, tx); lerr != nil {
			return lerr
		}
		var cerr error
		after, cerr = repo.CountAdminsTx(ctx, tx)
		return cerr
	}); terr != nil {
		t.Fatalf("lock + count (after assign): %v", terr)
	}
	if after != before+1 {
		t.Errorf("CountAdminsTx after assign: want %d, got %d", before+1, after)
	}
}

// TestUserRoleRepository_AdminLockAndCountTx_NilTx pins the nil-handle guards:
// both methods must return a wrapped error rather than dereferencing nil.
func TestUserRoleRepository_AdminLockAndCountTx_NilTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRoleRepository(testDB.GORM)

	if err := repo.AcquireAdminRoleLockTx(ctx, nil); err == nil {
		t.Error("AcquireAdminRoleLockTx(nil tx): want error, got nil")
	}
	if _, err := repo.CountAdminsTx(ctx, nil); err == nil {
		t.Error("CountAdminsTx(nil tx): want error, got nil")
	}
}
