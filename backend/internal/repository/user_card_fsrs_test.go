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

func TestUserCardFSRSRepository_UpsertTxAndFindByUserAndCardIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	card := newCard(cg.ID, "front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	now := time.Now().UTC().Truncate(time.Microsecond)
	first := domain.NewUserCardFSRSForNewCard(ownerID, card.ID, now)
	first.State.Reps = 1
	first.State.Due = now.Add(time.Hour)

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ucsRepo.UpsertTx(ctx, tx, first)
	}))

	got, err := ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 1, got[card.ID].State.Reps)
	require.True(t, got[card.ID].State.Due.Equal(first.State.Due))

	second := domain.NewUserCardFSRSForNewCard(ownerID, card.ID, now.Add(time.Minute))
	second.State.Reps = 2
	second.State.Lapses = 1
	second.State.Due = now.Add(24 * time.Hour)
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ucsRepo.UpsertTx(ctx, tx, second)
	}))

	got, err = ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 2, got[card.ID].State.Reps)
	require.Equal(t, 1, got[card.ID].State.Lapses)
	require.True(t, got[card.ID].State.Due.Equal(second.State.Due))
}

func TestUserCardFSRSRepository_FindByUserAndCardIDs_ScopesByViewer(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	card := newCard(cg.ID, "shared", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	now := time.Now().UTC().Truncate(time.Microsecond)
	ownerState := domain.NewUserCardFSRSForNewCard(ownerID, card.ID, now)
	ownerState.State.Reps = 1
	otherState := domain.NewUserCardFSRSForNewCard(otherUserID, card.ID, now)
	otherState.State.Reps = 7

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ucsRepo.UpsertTx(ctx, tx, ownerState); err != nil {
			return err
		}
		return ucsRepo.UpsertTx(ctx, tx, otherState)
	}))

	got, err := ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, 1, got[card.ID].State.Reps)
}

func TestUserCardFSRSRepository_FindByUserAndCardIDs_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	got, err := ucsRepo.FindByUserAndCardIDs(ctx, "00000000-0000-0000-0000-000000000001", nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}
