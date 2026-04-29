package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

func newCard(cardgroupID, front, back string) *domain.Card {
	now := time.Now().UTC()
	return &domain.Card{
		ID:          uuid.NewString(),
		CardgroupID: cardgroupID,
		Front:       front,
		Back:        back,
		FSRS:        domain.NewFSRSStateForNewCard(now),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func insertCardgroup(t *testing.T, ctx context.Context, ownerID string) *domain.Cardgroup {
	t.Helper()
	repo := repository.NewCardgroupRepository(testDB.GORM)
	cg := newCardgroup(ownerID, "Cards Repo Group")
	require.NoError(t, repo.Create(ctx, cg))
	return cg
}

func TestCardRepository_CRUD(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	card := newCard(cg.ID, "front", "back")
	require.NoError(t, repo.Create(ctx, card))

	got, err := repo.FindByID(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, card.ID, got.ID)
	require.Equal(t, cg.ID, got.CardgroupID)
	require.Equal(t, "front", got.Front)
	require.Equal(t, "back", got.Back)
	require.Equal(t, domain.FSRSStateNew, got.FSRS.State)
	require.Equal(t, 2.5, got.FSRS.Stability)
	require.Equal(t, 5.0, got.FSRS.Difficulty)

	time.Sleep(5 * time.Millisecond)
	front := "updated front"
	updated, err := repo.Update(ctx, card.ID, repository.CardUpdate{Front: &front})
	require.NoError(t, err)
	require.Equal(t, front, updated.Front)
	require.Equal(t, "back", updated.Back)
	require.True(t, updated.UpdatedAt.After(got.UpdatedAt))

	require.NoError(t, repo.Delete(ctx, card.ID))
	_, err = repo.FindByID(ctx, card.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound), "got %v", err)
}

func TestCardRepository_FindByCardgroup_Scoped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg1 := insertCardgroup(t, ctx, ownerID)
	cg2 := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	card1 := newCard(cg1.ID, "front 1", "back 1")
	card2 := newCard(cg2.ID, "front 2", "back 2")
	require.NoError(t, repo.Create(ctx, card1))
	require.NoError(t, repo.Create(ctx, card2))

	got, err := repo.FindByCardgroup(ctx, cg1.ID)
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, card := range got {
		ids[card.ID] = true
	}
	require.True(t, ids[card1.ID])
	require.False(t, ids[card2.ID])
}

func TestCardRepository_FindByIDs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	card1 := newCard(cg.ID, "front 1", "back 1")
	card2 := newCard(cg.ID, "front 2", "back 2")
	require.NoError(t, repo.Create(ctx, card1))
	require.NoError(t, repo.Create(ctx, card2))

	missing := uuid.NewString()
	got, err := repo.FindByIDs(ctx, []string{card1.ID, missing, card2.ID})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.NotNil(t, got[card1.ID])
	require.Nil(t, got[missing])
	require.NotNil(t, got[card2.ID])

	empty, err := repo.FindByIDs(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestCardRepository_TxFSRSMethods(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	dueCard := newCard(cg.ID, "due", "back")
	dueCard.FSRS.Due = time.Now().UTC().Add(-time.Hour)
	futureCard := newCard(cg.ID, "future", "back")
	futureCard.FSRS.Due = time.Now().UTC().Add(time.Hour)
	require.NoError(t, repo.Create(ctx, dueCard))
	require.NoError(t, repo.Create(ctx, futureCard))

	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		got, err := repo.FindByIDTx(ctx, tx, dueCard.ID)
		require.NoError(t, err)
		require.Equal(t, dueCard.ID, got.ID)

		state := got.FSRS
		state.Due = time.Now().UTC().Add(24 * time.Hour)
		state.Reps = 1
		require.NoError(t, repo.UpdateFSRSStateTx(ctx, tx, got.ID, state))

		due, err := repo.FindDueCardsTx(ctx, tx, cg.ID, time.Now().UTC(), 10)
		require.NoError(t, err)
		for _, card := range due {
			require.NotEqual(t, dueCard.ID, card.ID)
			require.NotEqual(t, futureCard.ID, card.ID)
		}
		return nil
	})
	require.NoError(t, err)

	updated, err := repo.FindByID(ctx, dueCard.ID)
	require.NoError(t, err)
	require.Equal(t, 1, updated.FSRS.Reps)
	require.True(t, updated.FSRS.Due.After(time.Now().UTC()))
}

func TestCardRepository_FindDueCardsTx_OrderedAndScoped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg1 := insertCardgroup(t, ctx, ownerID)
	cg2 := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	now := time.Now().UTC()

	later := newCard(cg1.ID, "later", "back")
	later.FSRS.Due = now.Add(-time.Hour)
	earlier := newCard(cg1.ID, "earlier", "back")
	earlier.FSRS.Due = now.Add(-2 * time.Hour)
	otherGroup := newCard(cg2.ID, "other", "back")
	otherGroup.FSRS.Due = now.Add(-3 * time.Hour)
	for _, card := range []*domain.Card{later, earlier, otherGroup} {
		require.NoError(t, repo.Create(ctx, card))
	}

	var due []*domain.Card
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		due, err = repo.FindDueCardsTx(ctx, tx, cg1.ID, now, 10)
		return err
	})
	require.NoError(t, err)
	require.Len(t, due, 2)
	require.Equal(t, earlier.ID, due[0].ID)
	require.Equal(t, later.ID, due[1].ID)
}

func TestCardRepository_OnCardgroupDeleteCascade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	cgRepo := repository.NewCardgroupRepository(testDB.GORM)

	card := newCard(cg.ID, "front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))
	require.NoError(t, cgRepo.Delete(ctx, cg.ID))

	_, err := cardRepo.FindByID(ctx, card.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound), "got %v", err)
}
