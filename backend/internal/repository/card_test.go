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
	_, err := repo.FindByIDTx(ctx, tx1, card.ID)
	require.NoError(t, err)

	tx2 := testDB.GORM.WithContext(ctx).Begin()
	require.NoError(t, tx2.Error)
	defer tx2.Rollback()
	var id string
	err = tx2.Raw("SELECT id FROM cards WHERE id = ? FOR UPDATE NOWAIT", card.ID).Scan(&id).Error
	require.Error(t, err, "second transaction should fail to acquire a NOWAIT lock")
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
	got, err := repo.FindByCardgroupAndFront(ctx, cg.ID, "find-front")
	if err != nil {
		t.Fatalf("FindByCardgroupAndFront (hit): %v", err)
	}
	if got.ID != card.ID {
		t.Errorf("FindByCardgroupAndFront (hit): ID = %q, want %q", got.ID, card.ID)
	}
	if got.Back != "find-back" {
		t.Errorf("FindByCardgroupAndFront (hit): Back = %q, want %q", got.Back, "find-back")
	}

	// Miss: unknown front value must return ErrNotFound.
	_, err = repo.FindByCardgroupAndFront(ctx, cg.ID, "no-such-front")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("FindByCardgroupAndFront (miss): want ErrNotFound, got %v", err)
	}
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

	got, err := repo.FindByCardgroupAndFront(ctx, cgB.ID, "apple")
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

	_, err := repo.FindByCardgroupAndFront(ctx, cg.ID, " apple ")
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
