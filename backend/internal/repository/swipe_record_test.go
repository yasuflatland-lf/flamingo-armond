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

func TestSwipeRecordRepository_ListRecentByUser_OrdersAndScopes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	swipeRepo := repository.NewSwipeRecordRepository(testDB.GORM)

	ownerCard1 := newCard(cg.ID, "owner 1", "back 1")
	ownerCard2 := newCard(cg.ID, "owner 2", "back 2")
	otherCard := newCard(cg.ID, "other", "back 3")
	require.NoError(t, cardRepo.Create(ctx, ownerCard1))
	require.NoError(t, cardRepo.Create(ctx, ownerCard2))
	require.NoError(t, cardRepo.Create(ctx, otherCard))

	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	earlier := base.Add(-time.Hour)
	sameReviewTime := base
	newest := base.Add(time.Hour)

	seed := func(id, userID, cardID string, reviewedAt time.Time) *domain.SwipeRecord {
		return &domain.SwipeRecord{
			ID:         id,
			UserID:     userID,
			CardID:     cardID,
			Rating:     domain.RatingEasy,
			ReviewedAt: reviewedAt,
			StateAfter: domain.NewFSRSStateForNewCard(reviewedAt),
		}
	}

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, sr := range []*domain.SwipeRecord{
			seed("00000000-0000-0000-0000-000000000001", ownerID, ownerCard1.ID, earlier),
			seed("00000000-0000-0000-0000-000000000002", ownerID, ownerCard1.ID, sameReviewTime),
			seed("00000000-0000-0000-0000-000000000003", ownerID, ownerCard2.ID, sameReviewTime),
			seed("00000000-0000-0000-0000-000000000004", ownerID, ownerCard2.ID, newest),
			seed("00000000-0000-0000-0000-000000000005", otherUserID, otherCard.ID, newest),
		} {
			if err := swipeRepo.CreateTx(ctx, tx, sr); err != nil {
				return err
			}
		}
		return nil
	}))

	got, err := swipeRepo.ListRecentByUser(ctx, ownerID, 10)
	require.NoError(t, err)
	require.Len(t, got, 4)
	require.Equal(t, "00000000-0000-0000-0000-000000000004", got[0].ID)
	require.Equal(t, "00000000-0000-0000-0000-000000000003", got[1].ID)
	require.Equal(t, "00000000-0000-0000-0000-000000000002", got[2].ID)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", got[3].ID)
	for _, sr := range got {
		require.Equal(t, ownerID, sr.UserID)
	}

	limited, err := swipeRepo.ListRecentByUser(ctx, ownerID, 2)
	require.NoError(t, err)
	require.Len(t, limited, 2)
	require.Equal(t, got[0].ID, limited[0].ID)
	require.Equal(t, got[1].ID, limited[1].ID)

	empty, err := swipeRepo.ListRecentByUser(ctx, ownerID, 0)
	require.NoError(t, err)
	require.Empty(t, empty)
}
