package repository_test

// TestMain, testDB, insertAuthUser, and the newCardgroup helper are defined in
// sibling files (user_test.go, cardgroup_test.go) and shared across this package.

import (
	"context"
	"testing"

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

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return repo.CreateTx(ctx, tx, cg)
	}))

	got, err := repo.FindByID(ctx, string(cg.ID))
	require.NoError(t, err)
	require.Equal(t, cg.ID, got.ID)
	require.Equal(t, ownerID, got.OwnerID, "owner_id is persisted")
	require.Equal(t, domain.CardgroupName("CreateTx Group"), got.Name, "name is persisted")
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
