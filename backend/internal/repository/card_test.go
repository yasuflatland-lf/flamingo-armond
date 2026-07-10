package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/repository"
)

func newCard(cardgroupID domain.CardgroupID, front, back string) *domain.Card {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &domain.Card{
		ID:          uuid.NewString(),
		CardgroupID: cardgroupID,
		Front:       domain.CardText(front),
		Back:        domain.CardText(back),
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
	require.Equal(t, domain.CardText("front"), got.Front)
	require.Equal(t, domain.CardText("back"), got.Back)

	time.Sleep(5 * time.Millisecond)
	front := "updated front"
	updated, err := repo.Update(ctx, card.ID, repository.CardUpdate{Front: &front})
	require.NoError(t, err)
	require.Equal(t, domain.CardText(front), updated.Front)
	require.Equal(t, domain.CardText("back"), updated.Back)
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

	got, err := repo.FindByCardgroup(ctx, string(cg1.ID))
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

func TestCardRepository_FindDueCards_UsesPerUserFSRSRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)

	dueCard := newCard(cg.ID, "due", "back")
	futureCard := newCard(cg.ID, "future", "back")
	require.NoError(t, repo.Create(ctx, dueCard))
	require.NoError(t, repo.Create(ctx, futureCard))

	// futureCard has THIS user's FSRS row scheduled tomorrow → not due.
	// dueCard has no row → surfaces through the new-card window.
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), futureCard.ID, now)
		state.State.Due = now.Add(24 * time.Hour)
		state.State.Reps = 1
		return ucsRepo.UpsertTx(ctx, tx, state)
	}))

	due, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now, 10)
	require.NoError(t, err)
	require.Equal(t, []string{dueCard.ID}, repoCardIDs(due))
}

// TestCardRepository_FindDueCards_IgnoresOtherUsersFSRSRows verifies that the
// LEFT JOIN is scoped to the calling user via ucs.user_id = ?. A second user's
// future-due FSRS row for the same card must not exclude that card from the
// calling user's new-card window (their own JOIN slot produces NULL, so the
// card falls through to the new window as expected).
func TestCardRepository_FindDueCards_IgnoresOtherUsersFSRSRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)

	card := newCard(cg.ID, "shared-card", "back")
	require.NoError(t, repo.Create(ctx, card))

	// Insert otherUser's FSRS row for card with due = tomorrow. If the JOIN
	// leaked other users' rows, this would push the card into the review
	// window (future due → not due for any user) or worse exclude it entirely.
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state := domain.NewUserCardFSRSForNewCard(domain.UserID(otherUserID), card.ID, now)
		state.State.Due = now.Add(24 * time.Hour)
		state.State.Reps = 1
		return ucsRepo.UpsertTx(ctx, tx, state)
	}))

	// ownerID has no FSRS row for card → the JOIN for ownerID returns NULL →
	// card surfaces through the new-card window.
	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now, 10)
	require.NoError(t, err)
	require.Equal(t, []string{card.ID}, repoCardIDs(got),
		"otherUser's future-due row must not hide the card from the calling user's new-card window")
}

func TestCardRepository_FindByIDTx_LocksRowForUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	card := newCard(cg.ID, "front", "back")
	require.NoError(t, repo.Create(ctx, card))

	tx1 := testDB.GORM.WithContext(ctx).Begin()
	require.NoError(t, tx1.Error)
	defer tx1.Rollback()
	_, err := repo.FindByIDForUpdateTx(ctx, tx1, card.ID)
	require.NoError(t, err)

	tx2 := testDB.GORM.WithContext(ctx).Begin()
	require.NoError(t, tx2.Error)
	defer tx2.Rollback()
	var id string
	err = tx2.Raw("SELECT id FROM cards WHERE id = ? FOR UPDATE NOWAIT", card.ID).Scan(&id).Error
	require.Error(t, err, "second transaction should fail to acquire a NOWAIT lock")
}

func TestCardRepository_FindDueCards_ScopedAndLimited(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg1 := insertCardgroup(t, ctx, ownerID)
	cg2 := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	// Fixtures carry LastReview == now (set by NewUserCardFSRSForNewCard); a
	// cutoff strictly after now keeps every due review inside the window.
	reviewedBefore := now.Add(time.Second)

	dueNow := newCard(cg1.ID, "due-now", "back")
	laterDue := newCard(cg1.ID, "later-due", "back")
	earlierDue := newCard(cg1.ID, "earlier-due", "back")
	future := newCard(cg1.ID, "future", "back")
	otherGroup := newCard(cg2.ID, "other-group", "back")
	for _, card := range []*domain.Card{dueNow, laterDue, earlierDue, future, otherGroup} {
		require.NoError(t, repo.Create(ctx, card))
	}
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for card, due := range map[*domain.Card]time.Time{
			dueNow:     now,
			laterDue:   now.Add(-time.Hour),
			earlierDue: now.Add(-2 * time.Hour),
			future:     now.Add(time.Hour),
			otherGroup: now.Add(-3 * time.Hour),
		} {
			state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now)
			state.State.Due = due
			if err := ucsRepo.UpsertTx(ctx, tx, state); err != nil {
				return err
			}
		}
		return nil
	}))

	// Selection within the review phase is random() now, so a small limit picks
	// some two of the three due review cards (never future / other-group).
	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg1.ID), now, reviewedBefore, 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	due3 := map[string]bool{earlierDue.ID: true, laterDue.ID: true, dueNow.ID: true}
	for _, id := range repoCardIDs(got) {
		require.True(t, due3[id], "limit=2 must select from the due review set, got %q", id)
	}

	got, err = repo.FindDueCardsForUser(ctx, ownerID, string(cg1.ID), now, reviewedBefore, 10)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{earlierDue.ID, laterDue.ID, dueNow.ID}, repoCardIDs(got))
	require.NotContains(t, repoCardIDs(got), otherGroup.ID, "FindDueCards must not leak cards from another cardgroup")

	empty, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg1.ID), now, reviewedBefore, 0)
	require.NoError(t, err)
	require.Empty(t, empty)
}

// TestCardRepository_FindDueCards_NoFSRSRow verifies that a card with no
// user_card_fsrs row is returned with State == FSRSPhaseNew and Due ==
// card.CreatedAt (the stable fallback).
func TestCardRepository_FindDueCards_NoFSRSRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	now := time.Now().UTC().Add(time.Hour) // far future so the card is "due"

	card := newCard(cg.ID, "no-fsrs", "back")
	require.NoError(t, repo.Create(ctx, card))

	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, card.ID, got[0].Card.ID)
	require.Equal(t, domain.FSRSPhaseNew, got[0].Phase)
	require.True(t, got[0].Due.Equal(card.CreatedAt), "Due should fall back to card.CreatedAt when no FSRS row exists")
}

// TestCardRepository_FindDueCards_FSRSStateMapping verifies that existing
// FSRS rows with Learning, Review, and Relearning states are mapped correctly
// to the corresponding domain.FSRSPhase values.
func TestCardRepository_FindDueCards_FSRSStateMapping(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)

	cases := []struct {
		front string
		state domain.FSRSPhase
	}{
		{"learning-card", domain.FSRSPhaseLearning},
		{"review-card", domain.FSRSPhaseReview},
		{"relearning-card", domain.FSRSPhaseRelearning},
	}

	cards := make([]*domain.Card, len(cases))
	for i, tc := range cases {
		c := newCard(cg.ID, tc.front, "back")
		require.NoError(t, repo.Create(ctx, c))
		cards[i] = c
	}

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, tc := range cases {
			ucs := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), cards[i].ID, now)
			ucs.State.Phase = tc.state
			ucs.State.Due = now.Add(-time.Minute) // ensure it is due
			if err := ucsRepo.UpsertTx(ctx, tx, ucs); err != nil {
				return err
			}
		}
		return nil
	}))

	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now.Add(time.Second), 10)
	require.NoError(t, err)
	require.Len(t, got, len(cases))

	byID := make(map[string]domain.DueCard, len(got))
	for _, dc := range got {
		byID[dc.Card.ID] = dc
	}
	for i, tc := range cases {
		dc, ok := byID[cards[i].ID]
		require.True(t, ok, "missing card for case %q", tc.front)
		require.Equal(t, tc.state, dc.Phase, "state mismatch for case %q", tc.front)
	}
}

// TestCardRepository_FindDueCards_InvalidState verifies that an out-of-range
// state value stored in user_card_fsrs causes FindDueCardsForUser to return an
// error rather than silently producing a zero-value FSRSPhase.
func TestCardRepository_FindDueCards_InvalidState(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)

	card := newCard(cg.ID, "invalid-state", "back")
	require.NoError(t, repo.Create(ctx, card))

	// Insert a user_card_fsrs row directly with an invalid state value (99) so
	// that the IsValid gate in findDueCardsOn is exercised.
	sqlDB, err := testDB.GORM.DB()
	require.NoError(t, err)
	_, err = sqlDB.ExecContext(ctx,
		`INSERT INTO user_card_fsrs
		 (user_id, card_id, state, due, stability, difficulty, reps, lapses, last_review, elapsed_days, scheduled_days, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 0, 5.0, 0, 0, $4, 0, 0, now(), now())`,
		ownerID, card.ID, 99, now.Add(-time.Minute),
	)
	require.NoError(t, err)

	_, err = repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now.Add(time.Second), 10)
	require.ErrorContains(t, err, "repository: card: invalid FSRSPhase 99")
}

func TestCardRepo_Create_DuplicateFront(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	first := newCard(cg.ID, "same-front", "back-one")
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create (first): %v", err)
	}

	second := newCard(cg.ID, "same-front", "back-two")
	err := repo.Create(ctx, second)
	if !errors.Is(err, repository.ErrCardDuplicateFront) {
		t.Fatalf("Create (duplicate): want ErrCardDuplicateFront, got %v", err)
	}
	// ErrCardDuplicateFront is a standalone "found" sentinel; must NOT match ErrNotFound.
	if errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("ErrCardDuplicateFront must not match ErrNotFound, got %v", err)
	}
}

func repoCardIDs(cards []domain.DueCard) []string {
	out := make([]string, len(cards))
	for i, dc := range cards {
		out[i] = dc.Card.ID
	}
	return out
}

func TestCardRepo_FindByCardgroupAndFront(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	card := newCard(cg.ID, "find-front", "find-back")
	if err := repo.Create(ctx, card); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Hit: existing (cardgroup_id, front) pair returns the card.
	got, err := repo.FindByCardgroupAndFront(ctx, string(cg.ID), "find-front")
	if err != nil {
		t.Fatalf("FindByCardgroupAndFront (hit): %v", err)
	}
	if got.ID != card.ID {
		t.Errorf("FindByCardgroupAndFront (hit): ID = %q, want %q", got.ID, card.ID)
	}
	if got.Back != "find-back" {
		t.Errorf("FindByCardgroupAndFront (hit): Back = %q, want %q", string(got.Back), "find-back")
	}

	// Miss: unknown front value must return ErrNotFound.
	_, err = repo.FindByCardgroupAndFront(ctx, string(cg.ID), "no-such-front")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("FindByCardgroupAndFront (miss): want ErrNotFound, got %v", err)
	}
}

func TestCardRepository_ListFrontsByCardgroupTx_Scoped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardRepository(testDB.GORM)

	cgA := insertCardgroup(t, ctx, ownerID)
	cgB := insertCardgroup(t, ctx, ownerID)
	require.NoError(t, repo.Create(ctx, newCard(cgA.ID, "banana", "back-a1")))
	require.NoError(t, repo.Create(ctx, newCard(cgA.ID, "apple", "back-a2")))
	require.NoError(t, repo.Create(ctx, newCard(cgB.ID, "carrot", "back-b1")))

	var fronts []string
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		fronts, txErr = repo.ListFrontsByCardgroupTx(ctx, tx, string(cgA.ID))
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, []string{"apple", "banana"}, fronts)
}

func TestCardRepository_DeleteByCardgroupAndFrontsTx_EmptySlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardRepository(testDB.GORM)
	cg := insertCardgroup(t, ctx, ownerID)
	card := newCard(cg.ID, "keep", "back")
	require.NoError(t, repo.Create(ctx, card))

	var affected int64
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		affected, txErr = repo.DeleteByCardgroupAndFrontsTx(ctx, tx, string(cg.ID), nil)
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), affected)

	got, err := repo.FindByID(ctx, card.ID)
	require.NoError(t, err)
	require.Equal(t, card.ID, got.ID)
}

func TestCardRepository_DeleteByCardgroupAndFrontsTx_ScopedDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardRepository(testDB.GORM)

	cgA := insertCardgroup(t, ctx, ownerID)
	cgB := insertCardgroup(t, ctx, ownerID)
	deleteA := newCard(cgA.ID, "shared", "back-a")
	keepA := newCard(cgA.ID, "keep-a", "back-a")
	keepB := newCard(cgB.ID, "shared", "back-b")
	require.NoError(t, repo.Create(ctx, deleteA))
	require.NoError(t, repo.Create(ctx, keepA))
	require.NoError(t, repo.Create(ctx, keepB))

	var affected int64
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		affected, txErr = repo.DeleteByCardgroupAndFrontsTx(ctx, tx, string(cgA.ID), []string{"shared"})
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	_, err = repo.FindByID(ctx, deleteA.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)
	_, err = repo.FindByID(ctx, keepA.ID)
	require.NoError(t, err)
	gotB, err := repo.FindByID(ctx, keepB.ID)
	require.NoError(t, err)
	require.Equal(t, keepB.ID, gotB.ID)
}

// TestCardRepository_DeleteByCardgroupAndFrontsTx_NonOverlappingFrontsScoped
// pins the cross-cardgroup scoping contract for the (cardgroup_id, front)
// natural-key delete: a delete scoped to cgB with a front that exists ONLY in
// cgA must not touch cgA's row. This complements the _ScopedDelete test, which
// covers the overlapping-front case; here the front is unique to the wrong
// cardgroup, which is the regression target if a future refactor drops the
// `cardgroup_id = ?` clause.
func TestCardRepository_DeleteByCardgroupAndFrontsTx_NonOverlappingFrontsScoped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardRepository(testDB.GORM)

	cgA := insertCardgroup(t, ctx, ownerID)
	cgB := insertCardgroup(t, ctx, ownerID)
	cardA := newCard(cgA.ID, "front-only-in-a", "back-a")
	require.NoError(t, repo.Create(ctx, cardA))

	var affected int64
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		affected, txErr = repo.DeleteByCardgroupAndFrontsTx(ctx, tx, string(cgB.ID), []string{"front-only-in-a"})
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), affected,
		"deleting via cgB must not match a row that lives in cgA")

	got, err := repo.FindByID(ctx, cardA.ID)
	require.NoError(t, err, "row owned by cgA must still exist")
	require.Equal(t, cardA.ID, got.ID)
}

func TestCardRepository_DeleteByCardgroupAndFrontsTx_DeleteByFronts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardRepository(testDB.GORM)
	cg := insertCardgroup(t, ctx, ownerID)

	cardA := newCard(cg.ID, "alpha", "back-a")
	cardB := newCard(cg.ID, "beta", "back-b")
	cardC := newCard(cg.ID, "gamma", "back-c")
	require.NoError(t, repo.Create(ctx, cardA))
	require.NoError(t, repo.Create(ctx, cardB))
	require.NoError(t, repo.Create(ctx, cardC))

	var affected int64
	err := testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var txErr error
		affected, txErr = repo.DeleteByCardgroupAndFrontsTx(ctx, tx, string(cg.ID), []string{"alpha", "gamma"})
		return txErr
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), affected)

	_, err = repo.FindByID(ctx, cardA.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)
	gotB, err := repo.FindByID(ctx, cardB.ID)
	require.NoError(t, err)
	require.Equal(t, cardB.ID, gotB.ID)
	_, err = repo.FindByID(ctx, cardC.ID)
	require.ErrorIs(t, err, repository.ErrNotFound)
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
	require.NoError(t, cgRepo.Delete(ctx, string(cg.ID)))

	_, err := cardRepo.FindByID(ctx, card.ID)
	require.True(t, errors.Is(err, repository.ErrNotFound), "got %v", err)
}

// TestCardRepo_FindByCardgroupAndFront_CardgroupScoped guards the
// cardgroup_id predicate in FindByCardgroupAndFront: a card with the same
// front value in a different cardgroup must not bleed through.
func TestCardRepo_FindByCardgroupAndFront_CardgroupScoped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardRepository(testDB.GORM)

	cgA := insertCardgroup(t, ctx, ownerID)
	cgB := insertCardgroup(t, ctx, ownerID)

	cardA := newCard(cgA.ID, "apple", "back-A")
	cardB := newCard(cgB.ID, "apple", "back-B")
	require.NoError(t, repo.Create(ctx, cardA))
	require.NoError(t, repo.Create(ctx, cardB))

	got, err := repo.FindByCardgroupAndFront(ctx, string(cgB.ID), "apple")
	require.NoError(t, err)
	require.Equal(t, cardB.ID, got.ID, "must return cardgroup B's card, not cardgroup A's")
}

// TestCardRepo_FindByCardgroupAndFront_TrimSensitive guards the exact-match
// contract: callers are responsible for trimming; the repo must not do TRIM /
// LOWER / LIKE matching. A front with surrounding whitespace must not match a
// stored value without it.
func TestCardRepo_FindByCardgroupAndFront_TrimSensitive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	card := newCard(cg.ID, "apple", "back")
	require.NoError(t, repo.Create(ctx, card))

	_, err := repo.FindByCardgroupAndFront(ctx, string(cg.ID), " apple ")
	require.True(t, errors.Is(err, repository.ErrNotFound),
		"padded front must not match exact-stored value; got %v", err)
}

// TestCardRepo_Create_OtherUniqueViolationNotMisclassified guards the
// ConstraintName check in Create: a 23505 violation on a constraint other than
// uq_cards_cardgroup_front (here: cards_pkey) must NOT be returned as
// ErrCardDuplicateFront. It must still surface as a non-nil error.
func TestCardRepo_Create_OtherUniqueViolationNotMisclassified(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cgA := insertCardgroup(t, ctx, ownerID)
	cgB := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	fixedID := uuid.NewString()
	first := newCard(cgA.ID, "front-pkey-1", "back-1")
	first.ID = fixedID
	require.NoError(t, repo.Create(ctx, first))

	// Reuse the same id with a different cardgroup and front to hit cards_pkey,
	// not uq_cards_cardgroup_front.
	second := newCard(cgB.ID, "front-pkey-2", "back-2")
	second.ID = fixedID
	err := repo.Create(ctx, second)

	require.Error(t, err, "duplicate primary key must return an error")
	require.False(t, errors.Is(err, repository.ErrCardDuplicateFront),
		"cards_pkey violation must not be classified as ErrCardDuplicateFront; got %v", err)

	// Confirm the underlying Postgres error code is 23505 so the test
	// exercises the intended code path.
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "underlying error must be a pgconn.PgError; got %T %v", err, err)
	require.Equal(t, "23505", pgErr.Code)
}

func strPtr(s string) *string { return &s }

// TestCardRepo_FindPageByCardgroup_Search verifies the search filter:
//   - hits on front substring
//   - hits on back substring
//   - miss when neither front nor back matches
//   - nil search behaves the same as no filter (all cards returned)
//   - empty string search skips the filter (all cards returned)
//   - LIKE metacharacters (%, _, \) in the search query are treated literally
//   - search is case-insensitive (ILIKE)
func TestCardRepo_FindPageByCardgroup_Search(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	// Insert cards with known front/back values for deterministic search results.
	cardApple := newCard(cg.ID, "apple pie", "a sweet dessert")
	cardBanana := newCard(cg.ID, "banana split", "COLD dessert with fruit")
	cardCherry := newCard(cg.ID, "100% cherry", "tart fruit")
	cardUnderscore := newCard(cg.ID, "a_b_c pattern", "matches underscore")
	cardBackslash := newCard(cg.ID, `back\slash`, "literal backslash front")
	require.NoError(t, repo.Create(ctx, cardApple))
	require.NoError(t, repo.Create(ctx, cardBanana))
	require.NoError(t, repo.Create(ctx, cardCherry))
	require.NoError(t, repo.Create(ctx, cardUnderscore))
	require.NoError(t, repo.Create(ctx, cardBackslash))

	ids := func(cards []*domain.Card) []string {
		out := make([]string, len(cards))
		for i, c := range cards {
			out[i] = c.ID
		}
		return out
	}
	contains := func(cards []*domain.Card, id string) bool {
		for _, c := range cards {
			if c.ID == id {
				return true
			}
		}
		return false
	}

	t.Run("hit on front substring", func(t *testing.T) {
		t.Parallel()
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr("apple"),
		)
		require.NoError(t, err)
		require.Equal(t, int64(1), total, "totalCount must reflect search filter")
		require.Len(t, got, 1)
		require.Equal(t, cardApple.ID, got[0].ID)
		_ = ids(got)
	})

	t.Run("hit on back substring", func(t *testing.T) {
		t.Parallel()
		// "dessert" appears in both apple and banana backs.
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr("dessert"),
		)
		require.NoError(t, err)
		require.Equal(t, int64(2), total)
		require.Len(t, got, 2)
		require.True(t, contains(got, cardApple.ID))
		require.True(t, contains(got, cardBanana.ID))
	})

	t.Run("miss: no match", func(t *testing.T) {
		t.Parallel()
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr("zzznomatch"),
		)
		require.NoError(t, err)
		require.Equal(t, int64(0), total)
		require.Empty(t, got)
	})

	t.Run("nil search returns all cards", func(t *testing.T) {
		t.Parallel()
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, nil,
		)
		require.NoError(t, err)
		require.Equal(t, int64(5), total)
		require.Len(t, got, 5)
	})

	// Defensive: the usecase normalizes empty/whitespace-only strings to nil
	// before reaching the repository, so this path is not a real-world caller.
	// The test is kept to verify the repository itself remains correct if ever
	// called directly (e.g. from tests or future non-GraphQL callers).
	t.Run("empty string search returns all cards (defensive)", func(t *testing.T) {
		t.Parallel()
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr(""),
		)
		require.NoError(t, err)
		require.Equal(t, int64(5), total)
		require.Len(t, got, 5)
	})

	t.Run("percent metachar treated literally", func(t *testing.T) {
		t.Parallel()
		// "100%" must only match cardCherry whose front contains that literal string.
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr("100%"),
		)
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Len(t, got, 1)
		require.Equal(t, cardCherry.ID, got[0].ID)
	})

	t.Run("underscore metachar treated literally", func(t *testing.T) {
		t.Parallel()
		// "a_b" must only match cardUnderscore, not every two-char prefix (LIKE _ = any single char).
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr("a_b"),
		)
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Len(t, got, 1)
		require.Equal(t, cardUnderscore.ID, got[0].ID)
	})

	t.Run("backslash metachar treated literally", func(t *testing.T) {
		t.Parallel()
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr(`back\slash`),
		)
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Len(t, got, 1)
		require.Equal(t, cardBackslash.ID, got[0].ID)
	})

	t.Run("case insensitive", func(t *testing.T) {
		t.Parallel()
		// "COLD" appears uppercase in banana's back; search with lowercase must still match.
		got, total, err := repo.FindPageByCardgroup(
			ctx, string(cg.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr("cold"),
		)
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Len(t, got, 1)
		require.Equal(t, cardBanana.ID, got[0].ID)
	})
}

// TestCardRepository_FindDueCards_ReviewRowsPrecedeNewRows proves the
// window-concat contract: rows from the review window always precede rows from
// the new-card window in the raw result, regardless of cards.position. The
// usecase OrderingPolicy applies the final interleave; the repository only
// guarantees the two windows are concatenated review-first.
func TestCardRepository_FindDueCards_ReviewRowsPrecedeNewRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)

	// reviewEarly: has an FSRS row with due = now-2h (earlier than now).
	// Position is set HIGH (100); position plays no role in window ordering.
	reviewEarly := newCard(cg.ID, "review-early", "back")
	reviewEarly.Position = 100
	require.NoError(t, repo.Create(ctx, reviewEarly))

	// newLowPos: no FSRS row, so it surfaces through the new-card window.
	// Position is LOW (0); it must still come AFTER the review row.
	newLowPos := newCard(cg.ID, "new-low-pos", "back")
	newLowPos.CreatedAt = now
	newLowPos.Position = 0
	require.NoError(t, repo.Create(ctx, newLowPos))

	// Upsert the FSRS row for reviewEarly so its effective due is now-2h.
	// NewUserCardFSRSForNewCard sets LastReview == now; a cutoff strictly after
	// now keeps the review row inside the window.
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), reviewEarly.ID, now)
		state.State.Due = now.Add(-2 * time.Hour)
		state.State.Reps = 1
		return ucsRepo.UpsertTx(ctx, tx, state)
	}))

	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now.Add(time.Second), 10)
	require.NoError(t, err)
	require.Equal(t,
		[]string{reviewEarly.ID, newLowPos.ID},
		repoCardIDs(got),
		"review-window rows always precede new-window rows in the raw result; position plays no role",
	)
}

// TestCardRepo_FindPageByCardgroup_Search_CrossTenantNonLeak verifies that a
// search applied to one cardgroup does not surface rows from another cardgroup
// even when both contain cards with the same front/back text. This is the
// cross-tenant test required by docs/backend/library-gotchas/repository-cross-tenant-negative-test.md.
func TestCardRepo_FindPageByCardgroup_Search_CrossTenantNonLeak(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	repo := repository.NewCardRepository(testDB.GORM)

	cgA := insertCardgroup(t, ctx, ownerID)
	cgB := insertCardgroup(t, ctx, ownerID)

	// Both cardgroups have a card with the exact same front text.
	cardA := newCard(cgA.ID, "shared front", "back-A")
	cardB := newCard(cgB.ID, "shared front", "back-B")
	require.NoError(t, repo.Create(ctx, cardA))
	require.NoError(t, repo.Create(ctx, cardB))

	// Query cgB with a search that matches the shared front.
	got, total, err := repo.FindPageByCardgroup(
		ctx, string(cgB.ID), nil, nil, 10, 0, repository.CardOrderByID, repository.SortAsc, strPtr("shared"),
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), total, "only cardgroup B's card should match")
	require.Len(t, got, 1)
	require.Equal(t, cardB.ID, got[0].ID, "must return cardgroup B's card, not cardgroup A's")
}

// TestCardRepository_FindDueCards_ReviewsNotStarvedByNewBacklog proves that a
// large backlog of new (never-reviewed) cards does not evict now-due FSRS review
// cards from the LIMIT window. Before the dual-fetch fix the combined query
// ordered by COALESCE(ucs.due, cards.created_at) ASC, so new cards (old
// created_at) always sorted ahead of review cards (newer due) and filled the
// limit, starving reviews on large cardgroups.
func TestCardRepository_FindDueCards_ReviewsNotStarvedByNewBacklog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)

	// Five new cards with an OLD created_at and no user_card_fsrs row. Under the
	// old combined ordering their created_at key sorts ahead of the reviews' due
	// key, so a small limit would return only these.
	oldCreated := now.Add(-240 * time.Hour)
	for i := 0; i < 5; i++ {
		c := newCard(cg.ID, "new-"+string(rune('0'+i)), "back")
		c.CreatedAt = oldCreated
		c.Position = i
		require.NoError(t, repo.Create(ctx, c))
	}

	// Two review cards: each has a user_card_fsrs row that is now due
	// (due <= now), with due timestamps NEWER than the new cards' created_at.
	reviewEarlier := newCard(cg.ID, "review-earlier", "back")
	reviewLater := newCard(cg.ID, "review-later", "back")
	require.NoError(t, repo.Create(ctx, reviewEarlier))
	require.NoError(t, repo.Create(ctx, reviewLater))
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for card, due := range map[*domain.Card]time.Time{
			reviewEarlier: now.Add(-2 * time.Hour),
			reviewLater:   now.Add(-1 * time.Hour),
		} {
			state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), card.ID, now)
			state.State.Due = due
			if err := ucsRepo.UpsertTx(ctx, tx, state); err != nil {
				return err
			}
		}
		return nil
	}))

	// limit=2 is smaller than the new-card backlog. The fix fetches reviews and
	// new cards in separate LIMIT windows, so both due reviews still surface.
	// NewUserCardFSRSForNewCard sets LastReview == now; a cutoff strictly after
	// now keeps both due reviews inside the window.
	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now.Add(time.Second), 2)
	require.NoError(t, err)
	ids := repoCardIDs(got)

	require.Contains(t, ids, reviewEarlier.ID,
		"due review card must not be starved by the new-card backlog")
	require.Contains(t, ids, reviewLater.ID,
		"due review card must not be starved by the new-card backlog")
	// Reviews are returned ahead of new cards; their in-phase order is random().
	require.GreaterOrEqual(t, len(ids), 2)
	require.ElementsMatch(t, []string{reviewEarlier.ID, reviewLater.ID}, ids[:2],
		"due reviews come first; in-phase selection order is random")
}

// TestCardRepository_FindDueCards_ExcludesCardsReviewedToday verifies the
// reviewedBefore cutoff: a due card whose last_review is at or after the
// boundary (i.e. swiped today) is excluded from the review window. The
// predicate is strict < so equality with the boundary also excludes the card.
func TestCardRepository_FindDueCards_ExcludesCardsReviewedToday(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	startOfToday := now.Add(-6 * time.Hour) // arbitrary boundary for the test

	reviewedYesterday := newCard(cg.ID, "reviewed-yesterday", "back")
	reviewedToday := newCard(cg.ID, "reviewed-today", "back")
	reviewedAtBoundary := newCard(cg.ID, "reviewed-at-boundary", "back")
	require.NoError(t, repo.Create(ctx, reviewedYesterday))
	require.NoError(t, repo.Create(ctx, reviewedToday))
	require.NoError(t, repo.Create(ctx, reviewedAtBoundary))

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		old := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), reviewedYesterday.ID, now)
		old.State.Phase = domain.FSRSPhaseLearning
		old.State.Due = now.Add(-time.Hour)
		old.State.LastReview = startOfToday.Add(-time.Hour) // before boundary → included
		if err := ucsRepo.UpsertTx(ctx, tx, old); err != nil {
			return err
		}
		fresh := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), reviewedToday.ID, now)
		fresh.State.Phase = domain.FSRSPhaseLearning
		fresh.State.Due = now.Add(-time.Hour)
		fresh.State.LastReview = startOfToday.Add(time.Hour) // after boundary → excluded
		if err := ucsRepo.UpsertTx(ctx, tx, fresh); err != nil {
			return err
		}
		// Exact-boundary case: last_review == startOfToday; predicate is strict <
		// so equality is false and the card must be excluded.
		boundary := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), reviewedAtBoundary.ID, now)
		boundary.State.Phase = domain.FSRSPhaseLearning
		boundary.State.Due = now.Add(-time.Hour)
		boundary.State.LastReview = startOfToday // exactly at boundary → excluded
		return ucsRepo.UpsertTx(ctx, tx, boundary)
	}))

	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, startOfToday, 10)
	require.NoError(t, err)
	require.Equal(t, []string{reviewedYesterday.ID}, repoCardIDs(got),
		"only cards reviewed strictly before the boundary enter the review window")
}

// TestCardRepository_FindDueCards_LearningPhaseWinsReviewSlots verifies that
// learning-phase (Again/Hard) rows outrank Review-state rows in the review
// window, both for SELECTION under a small limit and for ORDER in the
// returned slice (the pre-sort contract shuffleWithinPhase relies on).
func TestCardRepository_FindDueCards_LearningPhaseWinsReviewSlots(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)

	mk := func(front string, st domain.FSRSPhase) *domain.Card {
		c := newCard(cg.ID, front, "back")
		require.NoError(t, repo.Create(ctx, c))
		require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			s := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), c.ID, now)
			s.State.Phase = st
			s.State.Due = now.Add(-time.Hour)
			s.State.LastReview = now.Add(-24 * time.Hour)
			return ucsRepo.UpsertTx(ctx, tx, s)
		}))
		return c
	}
	l1 := mk("learning-1", domain.FSRSPhaseLearning)
	l2 := mk("relearning-1", domain.FSRSPhaseRelearning)
	r1 := mk("review-1", domain.FSRSPhaseReview)
	r2 := mk("review-2", domain.FSRSPhaseReview)

	// Selection: limit=2 must pick the two learning-phase rows.
	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now, 2)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{l1.ID, l2.ID}, repoCardIDs(got))

	// Order: with all four returned, learning-phase rows come first.
	got, err = repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now, 10)
	require.NoError(t, err)
	require.Len(t, got, 4)
	require.ElementsMatch(t, []string{l1.ID, l2.ID}, repoCardIDs(got)[:2])
	require.ElementsMatch(t, []string{r1.ID, r2.ID}, repoCardIDs(got)[2:])
}

// TestCardRepository_FindDueCards_SamplesNewCardsUnderLimit replaces the old
// position-order selection test: new cards are now sampled randomly, so the
// invariants are count, distinctness, and pool membership — not order.
func TestCardRepository_FindDueCards_SamplesNewCardsUnderLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	now := time.Now().UTC().Add(time.Hour)

	pool := make(map[string]bool, 5)
	for i := 0; i < 5; i++ {
		c := newCard(cg.ID, "new-"+string(rune('0'+i)), "back")
		require.NoError(t, repo.Create(ctx, c))
		pool[c.ID] = true
	}

	got, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, now, 3)
	require.NoError(t, err)
	require.Len(t, got, 3)
	seen := map[string]bool{}
	for _, dc := range got {
		require.True(t, pool[dc.Card.ID], "returned card must come from the pool")
		require.False(t, seen[dc.Card.ID], "no duplicates")
		seen[dc.Card.ID] = true
	}
}

// TestCardRepository_FindPracticeCards_BoundaryComplementarity is the critical
// pin for the practice window. It shares ONE boundary value between the learn
// window (FindDueCardsForUser) and the practice window (FindPracticeCardsForUser)
// and proves the two are complementary: a card reviewed before the boundary
// belongs to the learn queue, a card reviewed at-or-after the boundary belongs
// to the practice pool, and never-reviewed cards belong to neither side's
// last_review predicate (they surface only via the learn new-card window).
//
// Mutation-proof: flip the practice comparator `>=` to `>` in card.go and
// reviewedAtBoundary vanishes from the practice result, failing this test.
func TestCardRepository_FindPracticeCards_BoundaryComplementarity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	boundary := now.Add(-6 * time.Hour) // arbitrary start-of-day cutoff for the test

	reviewedYesterday := newCard(cg.ID, "reviewed-yesterday", "back")
	reviewedAtBoundary := newCard(cg.ID, "reviewed-at-boundary", "back")
	reviewedToday := newCard(cg.ID, "reviewed-today", "back")
	neverReviewed := newCard(cg.ID, "never-reviewed", "back")
	require.NoError(t, repo.Create(ctx, reviewedYesterday))
	require.NoError(t, repo.Create(ctx, reviewedAtBoundary))
	require.NoError(t, repo.Create(ctx, reviewedToday))
	require.NoError(t, repo.Create(ctx, neverReviewed))

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		yesterday := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), reviewedYesterday.ID, now)
		yesterday.State.Phase = domain.FSRSPhaseLearning
		yesterday.State.Due = now.Add(-time.Hour)                  // due has arrived
		yesterday.State.LastReview = boundary.Add(-24 * time.Hour) // before boundary → learn window
		if err := ucsRepo.UpsertTx(ctx, tx, yesterday); err != nil {
			return err
		}
		// Exact-boundary case: last_review == boundary. The practice predicate is
		// `>=` so equality is TRUE and the card is part of the practice pool. This
		// pins the inclusive comparator — flipping `>=` to `>` drops this card.
		atBoundary := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), reviewedAtBoundary.ID, now)
		atBoundary.State.Phase = domain.FSRSPhaseLearning
		atBoundary.State.Due = now.Add(-time.Hour)
		atBoundary.State.LastReview = boundary // exactly at boundary → practice pool (>=)
		if err := ucsRepo.UpsertTx(ctx, tx, atBoundary); err != nil {
			return err
		}
		today := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), reviewedToday.ID, now)
		today.State.Phase = domain.FSRSPhaseLearning
		today.State.Due = now.Add(-time.Hour)
		today.State.LastReview = boundary.Add(time.Hour) // after boundary → practice pool
		return ucsRepo.UpsertTx(ctx, tx, today)
	}))

	// Learn window: cards reviewed strictly before the boundary, plus the
	// never-reviewed card via the new-card window. reviewedAtBoundary and
	// reviewedToday are excluded (learn predicate is last_review < boundary).
	learn, err := repo.FindDueCardsForUser(ctx, ownerID, string(cg.ID), now, boundary, 10)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{reviewedYesterday.ID, neverReviewed.ID}, repoCardIDs(learn),
		"learn window holds cards reviewed before the boundary plus never-reviewed new cards")

	// Practice window: exactly the cards reviewed at-or-after the boundary.
	// reviewedYesterday (before boundary) and neverReviewed (NULL last_review)
	// are excluded. Result order is randomized, so compare order-insensitive.
	practice, err := repo.FindPracticeCardsForUser(ctx, ownerID, string(cg.ID), boundary, 10)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{reviewedAtBoundary.ID, reviewedToday.ID}, repoCardIDs(practice),
		"practice pool holds exactly the cards reviewed at-or-after the boundary (inclusive >=)")
}

// TestCardRepository_FindPracticeCards_IgnoresOtherUsersFSRSRows verifies the
// LEFT JOIN is scoped to the calling user via ucs.user_id = ?. Another user
// reviewing the shared card today must not surface it in the querying user's
// practice pool — the querying user has no FSRS row, so their JOIN slot is NULL
// and the last_review >= predicate never matches. Mirrors
// TestCardRepository_FindDueCards_IgnoresOtherUsersFSRSRows.
func TestCardRepository_FindPracticeCards_IgnoresOtherUsersFSRSRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	otherUserID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	boundary := now.Add(-6 * time.Hour)

	card := newCard(cg.ID, "shared-card", "back")
	require.NoError(t, repo.Create(ctx, card))

	// otherUser reviewed the card after the boundary (today). ownerID has no
	// FSRS row for the card.
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state := domain.NewUserCardFSRSForNewCard(domain.UserID(otherUserID), card.ID, now)
		state.State.LastReview = boundary.Add(time.Hour)
		state.State.Reps = 1
		return ucsRepo.UpsertTx(ctx, tx, state)
	}))

	got, err := repo.FindPracticeCardsForUser(ctx, ownerID, string(cg.ID), boundary, 10)
	require.NoError(t, err)
	require.Empty(t, got,
		"otherUser's today-review must not surface in the calling user's practice pool")
}

// TestCardRepository_FindPracticeCards_CardgroupScoped verifies the
// cards.cardgroup_id predicate: a card reviewed today in a DIFFERENT cardgroup
// of the same user must not leak into the queried cardgroup's practice pool.
func TestCardRepository_FindPracticeCards_CardgroupScoped(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg1 := insertCardgroup(t, ctx, ownerID)
	cg2 := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	boundary := now.Add(-6 * time.Hour)

	inGroup := newCard(cg1.ID, "in-group", "back")
	otherGroup := newCard(cg2.ID, "other-group", "back")
	require.NoError(t, repo.Create(ctx, inGroup))
	require.NoError(t, repo.Create(ctx, otherGroup))

	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range []*domain.Card{inGroup, otherGroup} {
			state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), c.ID, now)
			state.State.LastReview = boundary.Add(time.Hour) // reviewed today
			state.State.Reps = 1
			if err := ucsRepo.UpsertTx(ctx, tx, state); err != nil {
				return err
			}
		}
		return nil
	}))

	got, err := repo.FindPracticeCardsForUser(ctx, ownerID, string(cg1.ID), boundary, 10)
	require.NoError(t, err)
	require.Equal(t, []string{inGroup.ID}, repoCardIDs(got),
		"practice pool must not leak a today-reviewed card from another cardgroup")
}

// TestCardRepository_FindPracticeCards_Limited verifies the LIMIT clause and the
// limit<=0 short-circuit: with three reviewed-today cards a limit of 2 returns
// exactly two of the pool, and limit 0 returns an empty (non-nil) slice without
// executing SQL.
func TestCardRepository_FindPracticeCards_Limited(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)
	ucsRepo := repository.NewUserCardFSRSRepository(testDB.GORM)
	now := time.Now().UTC().Truncate(time.Microsecond)
	boundary := now.Add(-6 * time.Hour)

	pool := make(map[string]bool, 3)
	cards := make([]*domain.Card, 3)
	for i := 0; i < 3; i++ {
		c := newCard(cg.ID, "reviewed-"+string(rune('0'+i)), "back")
		require.NoError(t, repo.Create(ctx, c))
		cards[i] = c
		pool[c.ID] = true
	}
	require.NoError(t, testDB.GORM.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, c := range cards {
			state := domain.NewUserCardFSRSForNewCard(domain.UserID(ownerID), c.ID, now)
			state.State.LastReview = boundary.Add(time.Hour) // reviewed today
			state.State.Reps = 1
			if err := ucsRepo.UpsertTx(ctx, tx, state); err != nil {
				return err
			}
		}
		return nil
	}))

	got, err := repo.FindPracticeCardsForUser(ctx, ownerID, string(cg.ID), boundary, 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	for _, id := range repoCardIDs(got) {
		require.True(t, pool[id], "limit=2 must select from the reviewed-today pool, got %q", id)
	}

	empty, err := repo.FindPracticeCardsForUser(ctx, ownerID, string(cg.ID), boundary, 0)
	require.NoError(t, err)
	require.Empty(t, empty)
	require.NotNil(t, empty, "limit 0 returns an empty non-nil slice")
}

func TestCardRepo_CountExistingFronts_CaseSensitive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ownerID := insertAuthUser(t, ctx)
	cg := insertCardgroup(t, ctx, ownerID)
	repo := repository.NewCardRepository(testDB.GORM)

	// Seed two existing destination cards.
	apple := newCard(cg.ID, "Apple", "a")
	banana := newCard(cg.ID, "banana", "b")
	require.NoError(t, repo.Create(ctx, apple))
	require.NoError(t, repo.Create(ctx, banana))

	// "Apple" matches exactly; "apple" must NOT match (cards.front is case-sensitive
	// plain text, not citext); "cherry" is absent.
	n, err := repo.CountExistingFronts(ctx, string(cg.ID), []string{"Apple", "apple", "cherry"})
	require.NoError(t, err)
	require.Equal(t, int64(1), n, "only the exact-case 'Apple' overlaps")

	// Empty input is a no-op count of 0 (never a full scan).
	n0, err := repo.CountExistingFronts(ctx, string(cg.ID), []string{})
	require.NoError(t, err)
	require.Equal(t, int64(0), n0)
}
