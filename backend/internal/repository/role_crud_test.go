package repository_test

// Tests for RoleRepository Create, Update, Delete, and FindByID methods.
//
// TestMain, testDB, insertAuthUser, and sqlDBHandle are defined in user_test.go
// and shared across this package. insertRole is defined in user_role_assign_test.go.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestRoleRepository_FindByID_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	// "admin" is seeded by the initial migration.
	admin, err := repo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	got, err := repo.FindByID(ctx, admin.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.ID != admin.ID {
		t.Errorf("FindByID.ID = %q, want %q", got.ID, admin.ID)
	}
	if got.Name != "admin" {
		t.Errorf("FindByID.Name = %q, want admin", got.Name)
	}
}

func TestRoleRepository_FindByID_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	_, err := repo.FindByID(ctx, uuid.NewString())
	if !errors.Is(err, repository.ErrRoleNotFound) {
		t.Fatalf("FindByID(missing): want ErrRoleNotFound, got %v", err)
	}
	// ErrRoleNotFound is joined with ErrNotFound for backward compat.
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("FindByID(missing): want errors.Is(_, ErrNotFound) true, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FindByIDsTx
// ---------------------------------------------------------------------------

func TestRoleRepository_FindByIDsTx_ReturnsRolesInTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	roleAID := insertRole(t, ctx, "find-ids-tx-a-"+uuid.NewString())
	roleBID := insertRole(t, ctx, "find-ids-tx-b-"+uuid.NewString())
	missing := uuid.NewString()

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		got, err := repo.FindByIDsTx(ctx, tx, []string{roleAID, missing, roleBID})
		require.NoError(t, err)
		require.Len(t, got, 2)
		require.NotNil(t, got[roleAID])
		require.Nil(t, got[missing])
		require.NotNil(t, got[roleBID])
		return nil
	})
	require.NoError(t, err)
}

func TestRoleRepository_FindByIDsTx_NilTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	_, err := repo.FindByIDsTx(ctx, nil, []string{uuid.NewString()})
	require.Error(t, err)
}

// TestRoleRepository_FindByIDsTx_LocksRowsForUpdate proves FindByIDsTx acquires
// a FOR UPDATE row lock on the matched roles: a second transaction issuing a
// SELECT ... FOR UPDATE NOWAIT against the same row must fail to acquire the
// lock. Mirrors TestCardRepository_FindByIDForUpdateTx_LocksRowForUpdate.
func TestRoleRepository_FindByIDsTx_LocksRowsForUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	roleID := insertRole(t, ctx, "find-ids-tx-lock-"+uuid.NewString())

	tx1 := testDB.GORM.WithContext(ctx).Begin()
	require.NoError(t, tx1.Error)
	defer tx1.Rollback()
	got, err := repo.FindByIDsTx(ctx, tx1, []string{roleID})
	require.NoError(t, err)
	require.NotNil(t, got[roleID])

	tx2 := testDB.GORM.WithContext(ctx).Begin()
	require.NoError(t, tx2.Error)
	defer tx2.Rollback()
	var id string
	err = tx2.Raw("SELECT id FROM roles WHERE id = ? FOR UPDATE NOWAIT", roleID).Scan(&id).Error
	require.Error(t, err, "second transaction should fail to acquire a NOWAIT lock on the row held by FindByIDsTx")
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestRoleRepository_Create_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	name := "test-create-" + uuid.NewString()
	role, err := repo.Create(ctx, name)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if role == nil {
		t.Fatal("Create returned nil role")
	}
	if role.ID == "" {
		t.Error("Create returned role with empty ID")
	}
	if role.Name != domain.RoleName(name) {
		t.Errorf("Create.Name = %q, want %q", role.Name, name)
	}

	// Verify the row is actually in the DB via FindByID.
	fetched, err := repo.FindByID(ctx, role.ID)
	if err != nil {
		t.Fatalf("FindByID after Create: %v", err)
	}
	if fetched.Name != domain.RoleName(name) {
		t.Errorf("FindByID.Name = %q, want %q", fetched.Name, name)
	}
}

func TestRoleRepository_Create_NormalizesName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	// Provide a name with mixed case and surrounding whitespace.
	raw := "  MixedCase-" + uuid.NewString() + "  "
	role, err := repo.Create(ctx, raw)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	want := strings.ToLower(strings.TrimSpace(raw))
	if role.Name != domain.RoleName(want) {
		t.Errorf("Create.Name = %q, want %q (normalised)", role.Name, want)
	}
}

func TestRoleRepository_Create_Duplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	name := "dup-role-" + uuid.NewString()
	if _, err := repo.Create(ctx, name); err != nil {
		t.Fatalf("Create (first): %v", err)
	}

	_, err := repo.Create(ctx, name)
	if !errors.Is(err, repository.ErrRoleDuplicate) {
		t.Fatalf("Create (duplicate): want ErrRoleDuplicate, got %v", err)
	}
	// ErrRoleDuplicate must NOT match ErrNotFound.
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("ErrRoleDuplicate must not match ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestRoleRepository_Update_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	original := "update-before-" + uuid.NewString()
	created, err := repo.Create(ctx, original)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	newName := "update-after-" + uuid.NewString()
	updated, err := repo.Update(ctx, created.ID, newName)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.ID != created.ID {
		t.Errorf("Update.ID = %q, want %q", updated.ID, created.ID)
	}
	if updated.Name != domain.RoleName(newName) {
		t.Errorf("Update.Name = %q, want %q", updated.Name, newName)
	}

	// Verify persistence via FindByID.
	fetched, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID after Update: %v", err)
	}
	if fetched.Name != domain.RoleName(newName) {
		t.Errorf("FindByID.Name = %q after Update, want %q", fetched.Name, newName)
	}
}

func TestRoleRepository_Update_NormalizesName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	base := "upd-norm-" + uuid.NewString()
	created, err := repo.Create(ctx, base)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	raw := "  UpperCase-" + uuid.NewString() + "  "
	updated, err := repo.Update(ctx, created.ID, raw)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	want := strings.ToLower(strings.TrimSpace(raw))
	if updated.Name != domain.RoleName(want) {
		t.Errorf("Update.Name = %q, want %q (normalised)", updated.Name, want)
	}
}

func TestRoleRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	_, err := repo.Update(ctx, uuid.NewString(), "new-name")
	if !errors.Is(err, repository.ErrRoleNotFound) {
		t.Fatalf("Update(missing id): want ErrRoleNotFound, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("Update(missing id): want errors.Is(_, ErrNotFound) true, got %v", err)
	}
}

func TestRoleRepository_Update_Duplicate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	// Create two distinct roles.
	nameA := "upd-dup-a-" + uuid.NewString()
	nameB := "upd-dup-b-" + uuid.NewString()
	roleA, err := repo.Create(ctx, nameA)
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if _, err := repo.Create(ctx, nameB); err != nil {
		t.Fatalf("Create B: %v", err)
	}

	// Attempt to rename A to B's name — must return ErrRoleDuplicate.
	_, err = repo.Update(ctx, roleA.ID, nameB)
	if !errors.Is(err, repository.ErrRoleDuplicate) {
		t.Fatalf("Update to duplicate name: want ErrRoleDuplicate, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestRoleRepository_Delete_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	name := "delete-me-" + uuid.NewString()
	role, err := repo.Create(ctx, name)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, role.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// The row must no longer exist.
	_, err = repo.FindByID(ctx, role.ID)
	if !errors.Is(err, repository.ErrRoleNotFound) {
		t.Fatalf("FindByID after Delete: want ErrRoleNotFound, got %v", err)
	}
}

func TestRoleRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)

	err := repo.Delete(ctx, uuid.NewString())
	if !errors.Is(err, repository.ErrRoleNotFound) {
		t.Fatalf("Delete(missing id): want ErrRoleNotFound, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("Delete(missing id): want errors.Is(_, ErrNotFound) true, got %v", err)
	}
}

// TestRoleRepository_Delete_CascadesUserRoles verifies the ON DELETE CASCADE
// behaviour: deleting a role must also remove all user_roles rows that reference it.
func TestRoleRepository_Delete_CascadesUserRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewRoleRepository(testDB.GORM)
	userRoleRepo := repository.NewUserRoleRepository(testDB.GORM)

	// Create a role and a user, assign the role.
	name := "cascade-role-" + uuid.NewString()
	role, err := repo.Create(ctx, name)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	userID := insertAuthUser(t, ctx)
	if err := userRoleRepo.AssignRoleToUser(ctx, userID, role.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	// Delete the role — must succeed (CASCADE removes the user_roles row).
	if err := repo.Delete(ctx, role.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// The user should now have no roles.
	roles, err := userRoleRepo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser after cascaded Delete: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("ListByUser len after cascaded Delete = %d, want 0", len(roles))
	}
}
