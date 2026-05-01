package repository_test

// TestMain, testDB, insertAuthUser, and sqlDBHandle are defined in user_test.go
// and shared across this package.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"backend/internal/repository"
)

// insertRole inserts a new role directly into the roles table and returns
// the generated ID. Helper for tests that need roles beyond the seeded set.
func insertRole(t *testing.T, ctx context.Context, name string) string {
	t.Helper()
	sqlDB := sqlDBHandle(t)
	var id string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO public.roles (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id`,
		name,
	).Scan(&id); err != nil {
		t.Fatalf("insertRole %q: %v", name, err)
	}
	return id
}

// ---------------------------------------------------------------------------
// AssignToUser
// ---------------------------------------------------------------------------

func TestRoleRepository_AssignToUser_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewRoleRepository(testDB.GORM)

	admin, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	if err := repo.AssignToUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("AssignToUser: %v", err)
	}

	roles, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("ListByUser len = %d, want 1", len(roles))
	}
	if roles[0].ID != admin.ID {
		t.Fatalf("role ID = %q, want %q", roles[0].ID, admin.ID)
	}
}

func TestRoleRepository_AssignToUser_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewRoleRepository(testDB.GORM)

	admin, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	if err := repo.AssignToUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("AssignToUser (first): %v", err)
	}
	// Second call must not return an error.
	if err := repo.AssignToUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("AssignToUser (second, idempotent): %v", err)
	}

	roles, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("ListByUser len after double assign = %d, want 1", len(roles))
	}
}

func TestRoleRepository_AssignToUser_UserNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	admin, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	missingUser := uuid.NewString()
	err = repo.AssignToUser(ctx, missingUser, admin.ID)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("AssignToUser(missing user): want ErrNotFound, got %v", err)
	}
}

func TestRoleRepository_AssignToUser_RoleNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewRoleRepository(testDB.GORM)

	missingRole := uuid.NewString()
	err := repo.AssignToUser(ctx, userID, missingRole)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("AssignToUser(missing role): want ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RevokeFromUser
// ---------------------------------------------------------------------------

func TestRoleRepository_RevokeFromUser_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewRoleRepository(testDB.GORM)

	admin, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	if err := repo.AssignToUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("AssignToUser: %v", err)
	}
	if err := repo.RevokeFromUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("RevokeFromUser: %v", err)
	}

	roles, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser after revoke: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("ListByUser len after revoke = %d, want 0", len(roles))
	}
}

func TestRoleRepository_RevokeFromUser_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewRoleRepository(testDB.GORM)

	admin, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	// Revoke a row that was never assigned — must not return an error.
	if err := repo.RevokeFromUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("RevokeFromUser on missing row: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListByUser
// ---------------------------------------------------------------------------

func TestRoleRepository_ListByUser_NoRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewRoleRepository(testDB.GORM)

	roles, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if roles == nil {
		t.Fatal("ListByUser returned nil, want empty slice")
	}
	if len(roles) != 0 {
		t.Fatalf("ListByUser len = %d, want 0", len(roles))
	}
}

func TestRoleRepository_ListByUser_OrderedByNameAsc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewRoleRepository(testDB.GORM)

	// Seed three roles with names that should sort alphabetically.
	adminID := insertRole(t, ctx, "admin")
	generalID := insertRole(t, ctx, "general")
	reviewerID := insertRole(t, ctx, "reviewer")

	for _, roleID := range []string{reviewerID, adminID, generalID} {
		if err := repo.AssignToUser(ctx, userID, roleID); err != nil {
			t.Fatalf("AssignToUser(%s): %v", roleID, err)
		}
	}

	roles, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(roles) != 3 {
		t.Fatalf("ListByUser len = %d, want 3", len(roles))
	}

	want := []string{"admin", "general", "reviewer"}
	for i, r := range roles {
		if r.Name != want[i] {
			t.Errorf("roles[%d].Name = %q, want %q", i, r.Name, want[i])
		}
	}
}

// ---------------------------------------------------------------------------
// ListAll
// ---------------------------------------------------------------------------

// TestRoleRepository_ListAll_NameAscOrder inserts roles out of order and
// verifies that ListAll returns them sorted by name ASC.
func TestRoleRepository_ListAll_NameAscOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	// Seed roles with names that should sort alphabetically. "admin" and
	// "general" already exist in the test DB seed; add a unique one to
	// verify sorting includes it.
	_ = insertRole(t, ctx, "admin")
	_ = insertRole(t, ctx, "general")
	_ = insertRole(t, ctx, "zzz-test-sorter")

	roles, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if roles == nil {
		t.Fatal("ListAll returned nil, want non-nil slice")
	}
	// Verify the returned slice is non-empty and ordered.
	if len(roles) < 2 {
		t.Fatalf("ListAll len = %d, want >= 2", len(roles))
	}
	for i := 1; i < len(roles); i++ {
		if roles[i].Name < roles[i-1].Name {
			t.Errorf("ListAll not sorted ASC at [%d]: %q > %q", i, roles[i-1].Name, roles[i].Name)
		}
	}
}

// TestRoleRepository_ListAll_EmptySliceNotNil verifies that ListAll returns an
// empty non-nil slice rather than nil when the roles table has no rows.
// Because the shared test DB always has seed roles, this test works by
// asserting the return type contract rather than a true empty-table scenario —
// the important invariant is that the return is never nil regardless of row count.
func TestRoleRepository_ListAll_EmptySliceNotNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	roles, err := repo.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if roles == nil {
		t.Fatal("ListAll returned nil; want non-nil slice (empty or populated)")
	}
}
