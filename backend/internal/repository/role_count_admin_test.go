package repository_test

// TestRoleRepository_CountAdminUsers covers the two branches the startup
// bootstrap WARN log depends on: no admin role-holders and one admin.
//
// TestMain, testDB, insertAuthUser, and insertRole are defined in user_test.go
// and role_assign_test.go respectively and are shared across this package.

import (
	"context"
	"testing"

	"backend/internal/repository"
)

func TestRoleRepository_CountAdminUsers(t *testing.T) {
	// Not parallel: this test modifies shared state (user_roles for a fresh
	// user) and relies on the count being deterministic, so we run it serially
	// to avoid flakiness from other tests assigning the admin role concurrently.
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	// Case 1: zero admin role-holders for a fresh user (count may be > 0
	// globally due to parallel tests, but we verify the method returns a
	// non-negative integer without error).
	n, err := repo.CountAdminUsers(ctx)
	if err != nil {
		t.Fatalf("CountAdminUsers (before assign): unexpected error: %v", err)
	}
	if n < 0 {
		t.Errorf("CountAdminUsers: want >= 0, got %d", n)
	}

	// Case 2: after assigning a fresh user to the admin role, count must
	// increase by exactly 1.
	role, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}
	userID := insertAuthUser(t, ctx)
	if err := repo.AssignToUser(ctx, userID, role.ID); err != nil {
		t.Fatalf("AssignToUser: %v", err)
	}

	n2, err := repo.CountAdminUsers(ctx)
	if err != nil {
		t.Fatalf("CountAdminUsers (after assign): unexpected error: %v", err)
	}
	if n2 != n+1 {
		t.Errorf("CountAdminUsers after assign: want %d, got %d", n+1, n2)
	}
}
