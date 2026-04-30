package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/repository"
)

// TestCardRepository_BulkDelete covers DeleteByIDsTx across five scenarios.
func TestCardRepository_BulkDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty ids returns zero without touching DB", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)

		var affected int64
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			affected, txErr = repo.DeleteByIDsTx(ctx, tx, ownerID, []string{})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(0), affected)
	})

	t.Run("two own cards deleted returns two", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)

		card1 := newCard(cg.ID, "front1", "back1")
		card2 := newCard(cg.ID, "front2", "back2")
		require.NoError(t, repo.Create(ctx, card1))
		require.NoError(t, repo.Create(ctx, card2))

		var affected int64
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			affected, txErr = repo.DeleteByIDsTx(ctx, tx, ownerID, []string{card1.ID, card2.ID})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(2), affected)

		_, err1 := repo.FindByID(ctx, card1.ID)
		require.ErrorIs(t, err1, repository.ErrNotFound)
		_, err2 := repo.FindByID(ctx, card2.ID)
		require.ErrorIs(t, err2, repository.ErrNotFound)
	})

	t.Run("mix of own and foreign card deletes only own", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)

		ownerID := insertAuthUser(t, ctx)
		foreignOwnerID := insertAuthUser(t, ctx)

		ownCG := insertCardgroup(t, ctx, ownerID)
		foreignCG := insertCardgroup(t, ctx, foreignOwnerID)

		ownCard := newCard(ownCG.ID, "own", "back")
		foreignCard := newCard(foreignCG.ID, "foreign", "back")
		require.NoError(t, repo.Create(ctx, ownCard))
		require.NoError(t, repo.Create(ctx, foreignCard))

		var affected int64
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			affected, txErr = repo.DeleteByIDsTx(ctx, tx, ownerID, []string{ownCard.ID, foreignCard.ID})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)

		_, err1 := repo.FindByID(ctx, ownCard.ID)
		require.ErrorIs(t, err1, repository.ErrNotFound, "own card must be deleted")

		got, err2 := repo.FindByID(ctx, foreignCard.ID)
		require.NoError(t, err2, "foreign card must still exist")
		require.Equal(t, foreignCard.ID, got.ID)
	})

	t.Run("all-foreign ids returns zero and leaves rows intact", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)

		ownerID := insertAuthUser(t, ctx)
		foreignOwnerID := insertAuthUser(t, ctx)

		foreignCG := insertCardgroup(t, ctx, foreignOwnerID)
		foreignCard := newCard(foreignCG.ID, "foreign", "back")
		require.NoError(t, repo.Create(ctx, foreignCard))

		var affected int64
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			affected, txErr = repo.DeleteByIDsTx(ctx, tx, ownerID, []string{foreignCard.ID})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(0), affected)

		got, err2 := repo.FindByID(ctx, foreignCard.ID)
		require.NoError(t, err2)
		require.Equal(t, foreignCard.ID, got.ID)
	})

	t.Run("non-existent id mixed with own id deletes only own", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)

		ownCard := newCard(cg.ID, "own", "back")
		require.NoError(t, repo.Create(ctx, ownCard))

		nonExistentID := uuid.NewString()

		var affected int64
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			affected, txErr = repo.DeleteByIDsTx(ctx, tx, ownerID, []string{ownCard.ID, nonExistentID})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), affected)

		_, err1 := repo.FindByID(ctx, ownCard.ID)
		require.ErrorIs(t, err1, repository.ErrNotFound, "own card must be deleted")
	})
}
