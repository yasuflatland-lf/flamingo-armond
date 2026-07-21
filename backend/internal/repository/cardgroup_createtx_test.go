package repository_test

// TestMain, testDB, insertAuthUser, and the newCardgroup helper are defined in
// sibling files (user_test.go, cardgroup_test.go) and shared across this package.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// TestCardgroupRepository_CreateTx verifies the transaction-scoped insert path:
// a cardgroup created via CreateTx inside an explicit transaction lands with the
// supplied owner_id and name and is then readable through the non-tx FindByID.
func TestCardgroupRepository_CreateTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "CreateTx Group")
	cg.UpdatedAt = time.Unix(1, 0).UTC()

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.CreateTx(ctx, tx, cg)
	}))

	got, err := repo.FindByID(ctx, string(cg.ID))
	require.NoError(t, err)
	require.Equal(t, cg.ID, got.ID)
	require.Equal(t, ownerID, string(got.OwnerID), "owner_id is persisted")
	require.Equal(t, domain.CardgroupName("CreateTx Group"), got.Name, "name is persisted")
	require.Equal(t, cg.UpdatedAt, got.UpdatedAt,
		"CreateTx must copy the database-assigned updated_at back into the aggregate")
	require.NotEqual(t, time.Unix(1, 0).UTC(), cg.UpdatedAt)
}

// TestCardgroupRepository_CreateTx_DeletedOwner_ReturnsOwnerNotFound is the
// transaction-scoped mirror of the Create case: the master-deck copy paths
// (deck import and new-user starter seeding) write the caller-owned cardgroup
// through CreateTx, so a deleted account must reach the same classification
// there. The owner's auth.users row is deleted (cascading public.users away)
// while a still-valid JWT would keep authenticating them, so the in-transaction
// insert violates cardgroups_owner_id_fkey.
func TestCardgroupRepository_CreateTx_DeletedOwner_ReturnsOwnerNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	userRepo := repository.NewUserRepository(testDB.GORM)
	require.NoError(t, deleteAuthUserInTx(ctx, userRepo, ownerID))

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.CreateTx(ctx, tx, newCardgroup(ownerID, "Imported deck for a deleted account"))
	})
	require.ErrorIs(t, err, repository.ErrCardgroupOwnerNotFound)
	// Standalone sentinel: a missing owner must not read as a missing cardgroup,
	// or the import path would collapse it into the not-found outcome reserved
	// for an unpublished master.
	require.NotErrorIs(t, err, repository.ErrNotFound)
}

// TestCardgroupRepository_CreateTx_RollsBackOnError proves the insert
// participates in the caller's transaction: when the transaction function
// returns an error after the CreateTx insert, the row is rolled back and no
// cardgroup is left behind.
func TestCardgroupRepository_CreateTx_RollsBackOnError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardgroupRepository(testDB.GORM)

	cg := newCardgroup(ownerID, "Rollback Group")

	wantErr := context.Canceled
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repo.CreateTx(ctx, tx, cg); err != nil {
			return err
		}
		// Force a rollback after the insert.
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, err = repo.FindByID(ctx, string(cg.ID))
	require.ErrorIs(t, err, repository.ErrNotFound, "the insert was rolled back with the transaction")
}
