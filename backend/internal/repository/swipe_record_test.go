package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

func TestSwipeRecordRepository_CreateTxAndFind(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	card := newCard(cg.ID, "front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))
	reviewedAt := time.Now().UTC()
	state := domain.NewFSRSStateForNewCard(reviewedAt)
	state.Reps = 1
	sr, err := domain.NewSwipeRecord(ownerID, card.ID, domain.RatingEasy, reviewedAt, state)
	require.NoError(t, err)

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return swipeRepo.CreateTx(ctx, tx, sr)
	}))

	byID, err := swipeRepo.FindByIDs(ctx, []string{sr.ID})
	require.NoError(t, err)
	require.Len(t, byID, 1)
	require.Equal(t, sr.ID, byID[sr.ID].ID)
	require.Equal(t, domain.RatingEasy, byID[sr.ID].Rating)
	require.Equal(t, state.Reps, byID[sr.ID].StateAfter.Reps)

	history, err := swipeRepo.FindByUserAndCardgroup(ctx, ownerID, cg.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, sr.ID, history[0].ID)
}
