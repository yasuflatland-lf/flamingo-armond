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
	first := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now)
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

	second := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now.Add(time.Minute))
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

// TestUserCardFSRSRepository_OnCardDelete_CascadesFSRSRow proves the
// user_card_fsrs.card_id -> cards(id) FK is ON DELETE CASCADE: deleting a card
// removes every user's FSRS row for it. This is the runtime behaviour that
// idx_user_card_fsrs_card_id backs — the composite PK (user_id, card_id) cannot
// serve a card_id-only lookup, so without the index this cascade sequential-scans
// the whole table.
//   - SET NULL would violate the NOT NULL on user_card_fsrs.card_id.
//   - RESTRICT would block the card DELETE, breaking the cardgroup- and
//     user-deletion cascades that pass through cards.
func TestUserCardFSRSRepository_OnCardDelete_CascadesFSRSRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	cardRepo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	card := newCard(cg.ID, "front", "back")
	require.NoError(t, cardRepo.Create(ctx, card))

	now := time.Now().UTC().Truncate(time.Microsecond)
	state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now)
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return ucsRepo.UpsertTx(ctx, tx, state)
	}))

	// Precondition: the FSRS row exists before the card is deleted.
	got, err := ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 1, "expected the FSRS row to exist before card deletion")

	// Delete the card; the ON DELETE CASCADE FK on user_card_fsrs.card_id must
	// remove the dependent FSRS row.
	require.NoError(t, cardRepo.Delete(ctx, card.ID))

	got, err = ucsRepo.FindByUserAndCardIDs(ctx, ownerID, []string{card.ID})
	require.NoError(t, err)
	require.Len(t, got, 0, "FSRS row still exists after card deletion: FK is not ON DELETE CASCADE")
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
	ownerState := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now)
	ownerState.State.Reps = 1
	otherState := domain.NewUserCardFSRSForNewCard(domain.UserID(otherUserID), card.ID, now)
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

func TestUserCardFSRSRepository_FindByUserAndCardIDsTx_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)

	var got map[string]*domain.UserCardFSRS
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inner error
		got, inner = ucsRepo.FindByUserAndCardIDsTx(ctx, tx, "00000000-0000-0000-0000-000000000001", nil)
		return inner
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}
