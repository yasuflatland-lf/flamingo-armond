package repository_test

// TestMain, testDB, insertAuthUser, sqlDBHandle, and insertNAuthUsers are
// defined in user_test.go and shared across this package.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
)

// newCardgroup builds a minimal Cardgroup ready for Create.
func newCardgroup(ownerID, name string) *domain.Cardgroup {
	now := time.Now().UTC()
	return &domain.Cardgroup{
		ID:        uuid.NewString(),
		OwnerID:   ownerID,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestCardgroupRepository_CreateAndFindByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "My Flashcards")
	require.NoError(t, repo.Create(ctx, cg))

	got, err := repo.FindByID(ctx, cg.ID)
	require.NoError(t, err)
	require.Equal(t, cg.ID, got.ID)
	require.Equal(t, ownerID, got.OwnerID)
	require.Equal(t, "My Flashcards", got.Name)
	require.False(t, got.CreatedAt.IsZero())
	require.False(t, got.UpdatedAt.IsZero())
}

func TestCardgroupRepository_FindByOwner_ScopedToOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	owner1 := insertAuthUser(t, ctx)
	owner2 := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg1 := newCardgroup(owner1, "Owner1 Group")
	cg2 := newCardgroup(owner2, "Owner2 Group")
	require.NoError(t, repo.Create(ctx, cg1))
	require.NoError(t, repo.Create(ctx, cg2))

	result1, err := repo.FindByOwner(ctx, owner1)
	require.NoError(t, err)

	// Filter to only the groups we created in this test to avoid cross-test
	// interference from parallel tests that share the same DB.
	ids1 := cardgroupIDSet(result1)
	require.Contains(t, ids1, cg1.ID, "owner1 should see their own group")
	require.NotContains(t, ids1, cg2.ID, "owner1 should not see owner2's group")

	result2, err := repo.FindByOwner(ctx, owner2)
	require.NoError(t, err)
	ids2 := cardgroupIDSet(result2)
	require.Contains(t, ids2, cg2.ID, "owner2 should see their own group")
	require.NotContains(t, ids2, cg1.ID, "owner2 should not see owner1's group")
}

func cardgroupIDSet(cgs []*domain.Cardgroup) map[string]struct{} {
	m := make(map[string]struct{}, len(cgs))
	for _, cg := range cgs {
		m[cg.ID] = struct{}{}
	}
	return m
}

func TestCardgroupRepository_FindByOwner_OrderedByUpdatedAtDesc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg1 := newCardgroup(ownerID, "First")
	cg2 := newCardgroup(ownerID, "Second")
	cg3 := newCardgroup(ownerID, "Third")
	require.NoError(t, repo.Create(ctx, cg1))
	require.NoError(t, repo.Create(ctx, cg2))
	require.NoError(t, repo.Create(ctx, cg3))

	// Advance time so the trigger fires a strictly later updated_at.
	time.Sleep(5 * time.Millisecond)
	updated := "Second Updated"
	_, err := repo.Update(ctx, cg2.ID, repository.CardgroupUpdate{Name: &updated})
	require.NoError(t, err)

	result, err := repo.FindByOwner(ctx, ownerID)
	require.NoError(t, err)

	// Find positions of our three groups inside the (potentially larger) result.
	pos := make(map[string]int, 3)
	for i, cg := range result {
		switch cg.ID {
		case cg1.ID, cg2.ID, cg3.ID:
			pos[cg.ID] = i
		}
	}
	require.Len(t, pos, 3, "all three created cardgroups should be present")
	// cg2 was updated last, so it must appear before cg1 and cg3.
	require.Less(t, pos[cg2.ID], pos[cg1.ID], "updated group should sort before cg1")
	require.Less(t, pos[cg2.ID], pos[cg3.ID], "updated group should sort before cg3")
}

func TestCardgroupRepository_FindByIDs_AllFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg1 := newCardgroup(ownerID, "A")
	cg2 := newCardgroup(ownerID, "B")
	cg3 := newCardgroup(ownerID, "C")
	require.NoError(t, repo.Create(ctx, cg1))
	require.NoError(t, repo.Create(ctx, cg2))
	require.NoError(t, repo.Create(ctx, cg3))

	got, err := repo.FindByIDs(ctx, []string{cg1.ID, cg2.ID, cg3.ID})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.NotNil(t, got[cg1.ID])
	require.NotNil(t, got[cg2.ID])
	require.NotNil(t, got[cg3.ID])
}

func TestCardgroupRepository_FindByIDs_PartialMissing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Exists")
	require.NoError(t, repo.Create(ctx, cg))

	missing := uuid.NewString()
	got, err := repo.FindByIDs(ctx, []string{cg.ID, missing})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.NotNil(t, got[cg.ID])
	require.Nil(t, got[missing])
}

func TestCardgroupRepository_FindByIDs_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardgroupRepository(testDB.GORM)

	got, err := repo.FindByIDs(ctx, []string{})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestCardgroupRepository_Update_NameOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Original")
	require.NoError(t, repo.Create(ctx, cg))

	time.Sleep(5 * time.Millisecond)

	newName := "Renamed"
	got, err := repo.Update(ctx, cg.ID, repository.CardgroupUpdate{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, "Renamed", got.Name)
	require.True(t, got.UpdatedAt.After(cg.CreatedAt),
		"updated_at should be strictly later than created_at")
}

func TestCardgroupRepository_Update_EmptyPatchReturnsCurrent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Stable")
	require.NoError(t, repo.Create(ctx, cg))

	before, err := repo.FindByID(ctx, cg.ID)
	require.NoError(t, err)

	after, err := repo.Update(ctx, cg.ID, repository.CardgroupUpdate{})
	require.NoError(t, err)
	require.Equal(t, before.UpdatedAt.UnixNano(), after.UpdatedAt.UnixNano(),
		"empty patch must not bump updated_at")
	require.Equal(t, before.Name, after.Name)
}

func TestCardgroupRepository_Update_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardgroupRepository(testDB.GORM)

	name := "ghost"
	_, err := repo.Update(ctx, uuid.NewString(), repository.CardgroupUpdate{Name: &name})
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"want ErrNotFound, got %v", err)
}

func TestCardgroupRepository_Delete_Success(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "To Delete")
	require.NoError(t, repo.Create(ctx, cg))

	require.NoError(t, repo.Delete(ctx, cg.ID))

	_, err := repo.FindByID(ctx, cg.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"deleted cardgroup should not be found")
}

func TestCardgroupRepository_Delete_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardgroupRepository(testDB.GORM)

	err := repo.Delete(ctx, uuid.NewString())
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"want ErrNotFound for non-existent id, got %v", err)
}

func TestCardgroupRepository_OnUserDeleteCascade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Will Cascade")
	require.NoError(t, repo.Create(ctx, cg))

	// Deleting auth.users cascades → public.users → cardgroups.
	sqlDB := sqlDBHandle(t)
	_, err := sqlDB.ExecContext(ctx, `DELETE FROM auth.users WHERE id = $1`, ownerID)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, cg.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"cardgroup should be gone after cascade delete of owning user")
}

func TestCardgroupRepository_NameLengthCheckRejectsTooLong(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	// 101 ASCII characters: exceeds the CHECK constraint (1-100 trimmed code points).
	longName := strings.Repeat("a", 101)
	cg := newCardgroup(ownerID, longName)

	err := repo.Create(ctx, cg)
	require.Error(t, err, "DB CHECK constraint should reject a 101-char name")
}
