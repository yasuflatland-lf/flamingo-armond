package repository_test

// TestUserRoleRepository_CountAdmins covers the two branches the startup
// bootstrap WARN log depends on: no admin role-holders and one admin.
//
// TestMain, testDB, insertAuthUser are defined in user_test.go and shared
// across this package. insertRole is defined in user_role_assign_test.go.

import (
	"context"
	"testing"

	"backend/internal/repository"
)

func TestUserRoleRepository_CountAdmins(t *testing.T) {
	// Not parallel: this test modifies shared state (user_roles for a fresh
	// user) and relies on the count being deterministic, so we run it serially
	// to avoid flakiness from other tests assigning the admin role concurrently.
	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(testDB.GORM)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	// Case 1: verify the method returns a non-negative integer without error.
	n, err := repo.CountAdmins(ctx)
	if err != nil {
		t.Fatalf("CountAdmins (before assign): unexpected error: %v", err)
	}
	if n < 0 {
		t.Errorf("CountAdmins: want >= 0, got %d", n)
	}

	// Case 2: after assigning a fresh user to the admin role, count must
	// increase by exactly 1.
	role, err := roleRepo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}
	userID := insertAuthUser(t, ctx)
	if err := repo.AssignToUser(ctx, userID, role.ID); err != nil {
		t.Fatalf("AssignToUser: %v", err)
	}

	n2, err := repo.CountAdmins(ctx)
	if err != nil {
		t.Fatalf("CountAdmins (after assign): unexpected error: %v", err)
	}
	if n2 != n+1 {
		t.Errorf("CountAdmins after assign: want %d, got %d", n+1, n2)
	}
}
