package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

// TestCardRepository_UpsertManyTx covers the four scenarios for UpsertManyTx:
// pure inserts, mixed insert+update, empty input, and the per-cardgroup
// uniqueness boundary. The unique index that backs the ON CONFLICT clause is
// migration 20260503000000_add_cards_upsert_index.
func TestCardRepository_UpsertManyTx(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("all inserts", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)

		cards := []*domain.Card{
			newCard(cg.ID, "front-1", "back-1"),
			newCard(cg.ID, "front-2", "back-2"),
			newCard(cg.ID, "front-3", "back-3"),
			newCard(cg.ID, "front-4", "back-4"),
			newCard(cg.ID, "front-5", "back-5"),
		}

		var result repository.UpsertManyTxResult
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = repo.UpsertManyTx(ctx, tx, cards)
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(5), result.Inserted)
		require.Equal(t, int64(0), result.Updated)

		stored, err := repo.FindByCardgroup(ctx, string(cg.ID))
		require.NoError(t, err)
		require.Len(t, stored, 5)
	})

	t.Run("mixed inserts and updates", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)

		// Pre-insert three rows whose fronts will overlap with the upsert batch.
		pre1 := newCard(cg.ID, "shared-1", "old-back-1")
		pre2 := newCard(cg.ID, "shared-2", "old-back-2")
		pre3 := newCard(cg.ID, "shared-3", "old-back-3")
		for _, c := range []*domain.Card{pre1, pre2, pre3} {
			require.NoError(t, repo.Create(ctx, c))
		}

		// Upsert batch: 3 same fronts (with new backs) + 2 brand new fronts.
		// The IDs supplied here for the existing fronts MUST be ignored by the
		// ON CONFLICT path — the conflict key is (cardgroup_id, front).
		upsertBatch := []*domain.Card{
			newCard(cg.ID, "shared-1", "new-back-1"),
			newCard(cg.ID, "shared-2", "new-back-2"),
			newCard(cg.ID, "shared-3", "new-back-3"),
			newCard(cg.ID, "fresh-1", "fresh-back-1"),
			newCard(cg.ID, "fresh-2", "fresh-back-2"),
		}

		var result repository.UpsertManyTxResult
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = repo.UpsertManyTx(ctx, tx, upsertBatch)
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(2), result.Inserted)
		require.Equal(t, int64(3), result.Updated)

		// All three existing rows have their `back` text replaced; the row id
		// (the original PK) stays intact because ON CONFLICT only updates the
		// `back` and `updated_at` columns.
		for _, original := range []*domain.Card{pre1, pre2, pre3} {
			got, err := repo.FindByID(ctx, original.ID)
			require.NoError(t, err)
			require.Equal(t, original.Front, got.Front)
			require.NotEqual(t, original.Back, got.Back, "back must be overwritten")
		}

		// Final cardgroup row count: 3 pre-existing + 2 newly inserted.
		stored, err := repo.FindByCardgroup(ctx, string(cg.ID))
		require.NoError(t, err)
		require.Len(t, stored, 5)
	})

	t.Run("empty input is a no-op", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cg := insertCardgroup(t, ctx, ownerID)

		var result repository.UpsertManyTxResult
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = repo.UpsertManyTx(ctx, tx, nil)
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, repository.UpsertManyTxResult{}, result)

		stored, err := repo.FindByCardgroup(ctx, string(cg.ID))
		require.NoError(t, err)
		require.Empty(t, stored)
	})

	t.Run("same front in different cardgroups coexist", func(t *testing.T) {
		t.Parallel()
		repo := repository.NewCardRepository(testDB.GORM)
		ownerID := insertAuthUser(t, ctx)
		cgA := insertCardgroup(t, ctx, ownerID)
		cgB := insertCardgroup(t, ctx, ownerID)

		// Insert (group A, "hello").
		err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			_, txErr := repo.UpsertManyTx(ctx, tx, []*domain.Card{
				newCard(cgA.ID, "hello", "back-A"),
			})
			return txErr
		})
		require.NoError(t, err)

		// Upsert (group B, "hello") — must be classified as an INSERT because
		// the unique key is the (cardgroup_id, front) pair, not just `front`.
		var result repository.UpsertManyTxResult
		err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var txErr error
			result, txErr = repo.UpsertManyTx(ctx, tx, []*domain.Card{
				newCard(cgB.ID, "hello", "back-B"),
			})
			return txErr
		})
		require.NoError(t, err)
		require.Equal(t, int64(1), result.Inserted)
		require.Equal(t, int64(0), result.Updated)

		storedA, err := repo.FindByCardgroup(ctx, string(cgA.ID))
		require.NoError(t, err)
		require.Len(t, storedA, 1)
		require.Equal(t, domain.CardText("hello"), storedA[0].Front)
		require.Equal(t, domain.CardText("back-A"), storedA[0].Back)

		storedB, err := repo.FindByCardgroup(ctx, string(cgB.ID))
		require.NoError(t, err)
		require.Len(t, storedB, 1)
		require.Equal(t, domain.CardText("hello"), storedB[0].Front)
		require.Equal(t, domain.CardText("back-B"), storedB[0].Back)
	})
}

// TestCardRepository_UpsertManyTx_UpdatesPositionOnConflict proves the
// ON CONFLICT DO UPDATE path refreshes `position`. The first batch writes
// fronts F1,F2,F3 at positions 0,1,2; the second batch re-upserts the SAME
// fronts at positions 10,11,12. Because cardToDomain carries Position, a read
// after the second upsert must reflect the new positions.
func TestCardRepository_UpsertManyTx_UpdatesPositionOnConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repo := repository.NewCardRepository(testDB.GORM)
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)

	fronts := []string{"F1", "F2", "F3"}

	makeBatch := func(positions []int) []*domain.Card {
		batch := make([]*domain.Card, len(fronts))
		for i, front := range fronts {
			c := newCard(cg.ID, front, "back")
			c.Position = positions[i]
			batch[i] = c
		}
		return batch
	}

	// First batch: positions 0,1,2.
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, makeBatch([]int{0, 1, 2}))
		return txErr
	})
	require.NoError(t, err)

	for i, front := range fronts {
		got, err := repo.FindByCardgroupAndFront(ctx, string(cg.ID), front)
		require.NoError(t, err)
		require.Equal(t, i, got.Position, "initial position for %q", front)
	}

	// Second batch: SAME fronts, DIFFERENT positions 10,11,12. The conflict
	// path must refresh position.
	err = testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, txErr := repo.UpsertManyTx(ctx, tx, makeBatch([]int{10, 11, 12}))
		return txErr
	})
	require.NoError(t, err)

	for i, front := range fronts {
		got, err := repo.FindByCardgroupAndFront(ctx, string(cg.ID), front)
		require.NoError(t, err)
		require.Equal(t, 10+i, got.Position,
			"position for %q must be refreshed by the conflict path", front)
	}
}
