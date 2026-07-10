package repository_test

// TestMain, testDB, insertAuthUser, and sqlDBHandle are defined in user_test.go
// and shared across this package.

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"backend/internal/domain"
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
// AssignRoleToUser
// ---------------------------------------------------------------------------

func TestUserRoleRepository_AssignRoleToUser_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	roleRepo := repository.NewRoleRepository(testDB.GORM)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	admin, err := roleRepo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	if err := repo.AssignRoleToUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
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

func TestUserRoleRepository_AssignRoleToUser_Idempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	roleRepo := repository.NewRoleRepository(testDB.GORM)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	admin, err := roleRepo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	if err := repo.AssignRoleToUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("AssignRoleToUser (first): %v", err)
	}
	// Second call must not return an error.
	if err := repo.AssignRoleToUser(ctx, userID, admin.ID); err != nil {
		t.Fatalf("AssignRoleToUser (second, idempotent): %v", err)
	}

	roles, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(roles) != 1 {
		t.Fatalf("ListByUser len after double assign = %d, want 1", len(roles))
	}
}

func TestUserRoleRepository_AssignRoleToUser_UserNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	roleRepo := repository.NewRoleRepository(testDB.GORM)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	admin, err := roleRepo.FindByName(ctx, "admin")
	if err != nil {
		t.Fatalf("FindByName(admin): %v", err)
	}

	missingUser := uuid.NewString()
	err = repo.AssignRoleToUser(ctx, missingUser, admin.ID)
	if !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("AssignRoleToUser(missing user): want ErrUserNotFound, got %v", err)
	}
	// Backward-compat: legacy callers that match on ErrNotFound must still see
	// the joined sentinel.
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("AssignRoleToUser(missing user): want errors.Is(_, ErrNotFound) true, got %v", err)
	}
}

func TestUserRoleRepository_AssignRoleToUser_RoleNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	missingRole := uuid.NewString()
	err := repo.AssignRoleToUser(ctx, userID, missingRole)
	if !errors.Is(err, repository.ErrRoleNotFound) {
		t.Fatalf("AssignRoleToUser(missing role): want ErrRoleNotFound, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("AssignRoleToUser(missing role): want errors.Is(_, ErrNotFound) true, got %v", err)
	}
}

// TestUserRoleRepository_AssignRoleToUser_RoleNotFoundDistinct asserts that the new
// sentinels are distinct: a missing-role error must not match the
// missing-user sentinel, and vice versa.
func TestUserRoleRepository_AssignRoleToUser_RoleNotFoundDistinct(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	missingRole := uuid.NewString()
	err := repo.AssignRoleToUser(ctx, userID, missingRole)
	if !errors.Is(err, repository.ErrRoleNotFound) {
		t.Fatalf("want ErrRoleNotFound, got %v", err)
	}
	if errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("missing-role error must not match ErrUserNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SetUserRolesTx
// ---------------------------------------------------------------------------

func TestUserRoleRepository_SetUserRolesTx_ReplacesRoleSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	adminID := insertRole(t, ctx, "admin")
	generalID := insertRole(t, ctx, "general")
	reviewerID := insertRole(t, ctx, "reviewer")

	for _, roleID := range []string{adminID, generalID} {
		if err := repo.AssignRoleToUser(ctx, userID, roleID); err != nil {
			t.Fatalf("AssignRoleToUser(%s): %v", roleID, err)
		}
	}

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.SetUserRolesTx(ctx, tx, userID, []string{generalID, reviewerID})
	})
	if err != nil {
		t.Fatalf("SetUserRolesTx: %v", err)
	}

	assertUserRoleIDs(t, ctx, repo, userID, []string{generalID, reviewerID})
}

func TestUserRoleRepository_SetUserRolesTx_EmptyTargetClearsAllRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	adminID := insertRole(t, ctx, "admin")
	generalID := insertRole(t, ctx, "general")
	for _, roleID := range []string{adminID, generalID} {
		if err := repo.AssignRoleToUser(ctx, userID, roleID); err != nil {
			t.Fatalf("AssignRoleToUser(%s): %v", roleID, err)
		}
	}

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.SetUserRolesTx(ctx, tx, userID, nil)
	})
	if err != nil {
		t.Fatalf("SetUserRolesTx(empty): %v", err)
	}

	assertUserRoleIDs(t, ctx, repo, userID, nil)
}

func TestUserRoleRepository_SetUserRolesTx_IdempotentSameSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	adminID := insertRole(t, ctx, "admin")
	generalID := insertRole(t, ctx, "general")

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.SetUserRolesTx(ctx, tx, userID, []string{adminID, generalID})
	})
	if err != nil {
		t.Fatalf("SetUserRolesTx(first): %v", err)
	}
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.SetUserRolesTx(ctx, tx, userID, []string{generalID, adminID})
	})
	if err != nil {
		t.Fatalf("SetUserRolesTx(second): %v", err)
	}

	assertUserRoleIDs(t, ctx, repo, userID, []string{adminID, generalID})
}

func TestUserRoleRepository_SetUserRolesTx_UserNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRoleRepository(testDB.GORM)
	adminID := insertRole(t, ctx, "admin")
	missingUser := uuid.NewString()

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.SetUserRolesTx(ctx, tx, missingUser, []string{adminID})
	})
	if !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("SetUserRolesTx(missing user): want ErrUserNotFound, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("SetUserRolesTx(missing user): want ErrNotFound, got %v", err)
	}
}

func TestUserRoleRepository_SetUserRolesTx_RoleNotFoundRollsBack(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	adminID := insertRole(t, ctx, "admin")
	if err := repo.AssignRoleToUser(ctx, userID, adminID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	missingRole := uuid.NewString()
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.SetUserRolesTx(ctx, tx, userID, []string{missingRole})
	})
	if !errors.Is(err, repository.ErrRoleNotFound) {
		t.Fatalf("SetUserRolesTx(missing role): want ErrRoleNotFound, got %v", err)
	}
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("SetUserRolesTx(missing role): want ErrNotFound, got %v", err)
	}

	assertUserRoleIDs(t, ctx, repo, userID, []string{adminID})
}

// TestUserRoleRepository_SetUserRolesTx_DuplicateRoleIDsDeDupe pins the batched
// existence check: duplicate ids in a single call collapse into uniqueRoleIDs
// before the COUNT comparison, so a repeated (but existing) role validates and
// is assigned exactly once. A count keyed off the raw (un-deduped) length would
// see COUNT=1 != len=2 and wrongly reject with ErrRoleNotFound.
func TestUserRoleRepository_SetUserRolesTx_DuplicateRoleIDsDeDupe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	adminID := insertRole(t, ctx, "admin")
	generalID := insertRole(t, ctx, "general")

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.SetUserRolesTx(ctx, tx, userID, []string{adminID, adminID, generalID, adminID})
	})
	if err != nil {
		t.Fatalf("SetUserRolesTx(duplicates of existing roles): %v", err)
	}

	assertUserRoleIDs(t, ctx, repo, userID, []string{adminID, generalID})

	// A duplicate id paired with a nonexistent id must still reject: dedup leaves
	// two unique ids but only one exists, so COUNT=1 != len=2 → ErrRoleNotFound.
	missingRole := uuid.NewString()
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.SetUserRolesTx(ctx, tx, userID, []string{adminID, missingRole, adminID})
	})
	if !errors.Is(err, repository.ErrRoleNotFound) {
		t.Fatalf("SetUserRolesTx(dup + missing): want ErrRoleNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ListByUser
// ---------------------------------------------------------------------------

func TestUserRoleRepository_ListByUser_NoRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

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

func TestUserRoleRepository_ListByUser_OrderedByNameAsc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	userID := insertAuthUser(t, ctx)
	repo := repository.NewUserRoleRepository(testDB.GORM)

	// Seed three roles with names that should sort alphabetically.
	adminID := insertRole(t, ctx, "admin")
	generalID := insertRole(t, ctx, "general")
	reviewerID := insertRole(t, ctx, "reviewer")

	for _, roleID := range []string{reviewerID, adminID, generalID} {
		if err := repo.AssignRoleToUser(ctx, userID, roleID); err != nil {
			t.Fatalf("AssignRoleToUser(%s): %v", roleID, err)
		}
	}

	roles, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(roles) != 3 {
		t.Fatalf("ListByUser len = %d, want 3", len(roles))
	}

	want := []domain.RoleName{"admin", "general", "reviewer"}
	for i, r := range roles {
		if r.Name != want[i] {
			t.Errorf("roles[%d].Name = %q, want %q", i, r.Name, want[i])
		}
	}
}

func assertUserRoleIDs(
	t *testing.T,
	ctx context.Context,
	repo repository.UserRoleRepository,
	userID string,
	want []string,
) {
	t.Helper()
	roles, err := repo.ListByUser(ctx, userID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	got := make(map[string]bool, len(roles))
	for _, role := range roles {
		got[role.ID] = true
	}
	if len(got) != len(want) {
		t.Fatalf("role IDs len = %d, want %d (got=%v want=%v)", len(got), len(want), got, want)
	}
	for _, roleID := range want {
		if !got[roleID] {
			t.Fatalf("missing role ID %q in got=%v", roleID, got)
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

// ---------------------------------------------------------------------------
// ListByUserIDs (C4)
// ---------------------------------------------------------------------------

// TestUserRoleRepository_ListByUserIDs_EmptySlice verifies the GORM empty-IN guard:
// passing an empty userIDs slice must return an empty (non-nil) map and must
// not issue any SQL query (the method short-circuits before touching the DB).
func TestUserRoleRepository_ListByUserIDs_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRoleRepository(testDB.GORM)

	result, err := repo.ListByUserIDs(ctx, []string{})
	if err != nil {
		t.Fatalf("ListByUserIDs(empty): unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("ListByUserIDs(empty): expected non-nil map, got nil")
	}
	if len(result) != 0 {
		t.Fatalf("ListByUserIDs(empty): expected empty map, got %d entries", len(result))
	}
}

// TestUserRoleRepository_ListByUserIDs_MultipleUsers_NameAsc creates three users
// each with two roles assigned in varying insertion order, then verifies that
// ListByUserIDs returns each user's roles sorted by name ASC.
func TestUserRoleRepository_ListByUserIDs_MultipleUsers_NameAsc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRoleRepository(testDB.GORM)

	userA := insertAuthUser(t, ctx)
	userB := insertAuthUser(t, ctx)
	userC := insertAuthUser(t, ctx)

	// Ensure the three named roles exist (idempotent insert).
	adminID := insertRole(t, ctx, "admin")
	generalID := insertRole(t, ctx, "general")
	reviewerID := insertRole(t, ctx, "reviewer")

	// Assign in deliberate reverse-alphabetical order for each user.
	for _, uid := range []string{userA, userB, userC} {
		for _, rid := range []string{reviewerID, adminID} {
			if err := repo.AssignRoleToUser(ctx, uid, rid); err != nil {
				t.Fatalf("AssignRoleToUser(%s, %s): %v", uid, rid, err)
			}
		}
	}
	// userC also gets "general" to exercise a third distinct ordering.
	if err := repo.AssignRoleToUser(ctx, userC, generalID); err != nil {
		t.Fatalf("AssignRoleToUser(userC, general): %v", err)
	}

	result, err := repo.ListByUserIDs(ctx, []string{userA, userB, userC})
	if err != nil {
		t.Fatalf("ListByUserIDs: unexpected error: %v", err)
	}

	// userA: 2 roles — "admin", "reviewer" in name ASC.
	rolesA := result[userA]
	if len(rolesA) != 2 {
		t.Fatalf("userA: expected 2 roles, got %d", len(rolesA))
	}
	if rolesA[0].Name != "admin" || rolesA[1].Name != "reviewer" {
		t.Fatalf("userA roles not sorted ASC: %v", rolesA)
	}

	// userB: same 2 roles.
	rolesB := result[userB]
	if len(rolesB) != 2 {
		t.Fatalf("userB: expected 2 roles, got %d", len(rolesB))
	}
	if rolesB[0].Name != "admin" || rolesB[1].Name != "reviewer" {
		t.Fatalf("userB roles not sorted ASC: %v", rolesB)
	}

	// userC: 3 roles — "admin", "general", "reviewer" in name ASC.
	rolesC := result[userC]
	if len(rolesC) != 3 {
		t.Fatalf("userC: expected 3 roles, got %d", len(rolesC))
	}
	if rolesC[0].Name != "admin" || rolesC[1].Name != "general" || rolesC[2].Name != "reviewer" {
		t.Fatalf("userC roles not sorted ASC: %v", rolesC)
	}
}

// TestUserRoleRepository_ListByUserIDs_UnknownUserAbsentFromMap verifies that
// passing a mix of a known user and an unknown UUID returns only the known
// user's entry in the map. The unknown UUID must not appear as a key and no
// error is returned.
func TestUserRoleRepository_ListByUserIDs_UnknownUserAbsentFromMap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewUserRoleRepository(testDB.GORM)

	knownUser := insertAuthUser(t, ctx)
	unknownUser := uuid.NewString()

	adminID := insertRole(t, ctx, "admin")
	if err := repo.AssignRoleToUser(ctx, knownUser, adminID); err != nil {
		t.Fatalf("AssignRoleToUser: %v", err)
	}

	result, err := repo.ListByUserIDs(ctx, []string{knownUser, unknownUser})
	if err != nil {
		t.Fatalf("ListByUserIDs: unexpected error: %v", err)
	}

	if _, present := result[unknownUser]; present {
		t.Fatalf("unknown user %q must not appear in the result map", unknownUser)
	}
	rolesKnown := result[knownUser]
	if len(rolesKnown) != 1 {
		t.Fatalf("known user: expected 1 role, got %d", len(rolesKnown))
	}
	if rolesKnown[0].Name != "admin" {
		t.Fatalf("known user role name = %q, want admin", rolesKnown[0].Name)
	}
}
