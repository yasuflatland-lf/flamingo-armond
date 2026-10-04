package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/repository"
)

type mockSwipeRecordRepoForSwipe struct {
	createErr error
	created   *domain.SwipeRecord
	// phaseBeforeAtCreate is a value snapshot of created.PhaseBefore taken at
	// CreateTx call time, so an "X before Y" ordering assertion sees the
	// pre-rating phase rather than a post-hoc read of a possibly-mutated state
	// (docs/backend/library-gotchas/mock-snapshot-for-ordering-assertion.md).
	phaseBeforeAtCreate domain.FSRSPhase
	phaseAfterAtCreate  *domain.FSRSPhase
	// stabilityBeforeAtCreate is the same kind of value snapshot for
	// created.StabilityBefore.
	stabilityBeforeAtCreate float64
}

func (m *mockSwipeRecordRepoForSwipe) CreateTx(_ context.Context, _ *gorm.DB, sr *domain.SwipeRecord) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = sr
	if sr != nil {
		m.phaseBeforeAtCreate = sr.PhaseBefore
		m.stabilityBeforeAtCreate = sr.StabilityBefore
		after := sr.StateAfter.Phase
		m.phaseAfterAtCreate = &after
	}
	return nil
}

func TestSwipeUsecase_HandleSwipeCreatesUserFSRSStateForFirstSwipe(t *testing.T) {
	t.Parallel()

	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-1"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	userFSRSRepo := &mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}}
	tx := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		swipeRepo,
		service.NewFSRSScheduler(),
		tx,
		userFSRSRepo,
		newTestLogger(),
	)

	outcome, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Swipe == nil {
		t.Fatal("expected non-nil Swipe on success")
	}
	if outcome.Swipe.CardID != "card-1" {
		t.Fatalf("Swipe.CardID=%q, want card-1", outcome.Swipe.CardID)
	}
	if userFSRSRepo.upserted == nil {
		t.Fatal("expected per-user FSRS row to be upserted")
	}
	if userFSRSRepo.upserted.UserID != "user-1" || userFSRSRepo.upserted.CardID != "card-1" {
		t.Fatalf("unexpected upserted identity: %+v", userFSRSRepo.upserted)
	}
	if userFSRSRepo.upserted.State.Reps != 1 {
		t.Fatalf("expected first swipe to produce reps=1, got %+v", userFSRSRepo.upserted.State)
	}
	if swipeRepo.created == nil || swipeRepo.created.StateAfter != userFSRSRepo.upserted.State {
		t.Fatalf("swipe snapshot must match upserted user state, swipe=%+v ucs=%+v", swipeRepo.created, userFSRSRepo.upserted)
	}
	if swipeRepo.created.CardgroupID != "cg-1" {
		t.Fatalf("created swipe cardgroup=%q, want cg-1", swipeRepo.created.CardgroupID)
	}
}

// TestSwipeUsecase_HandleSwipe_RecordsPreRatingPhase proves the swipe record
// handed to CreateTx carries the phase the card was in BEFORE applyRating
// mutated it, not the post-rating phase. For a brand-new card the pre-swipe
// phase is FSRSPhaseNew; a RatingEasy graduation advances StateAfter.Phase past
// New, so asserting PhaseBefore == New AND StateAfter.Phase != New pins that the
// snapshot was captured before the mutation. The same argument pins
// StabilityBefore against the new-card starting stability. The mock snapshots
// both at CreateTx call time
// (docs/backend/library-gotchas/mock-snapshot-for-ordering-assertion.md).
func TestSwipeUsecase_HandleSwipe_RecordsPreRatingPhase(t *testing.T) {
	t.Parallel()

	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-1"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	userFSRSRepo := &mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}}
	tx := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		swipeRepo,
		service.NewFSRSScheduler(),
		tx,
		userFSRSRepo,
		newTestLogger(),
	)

	outcome, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Swipe == nil {
		t.Fatal("expected non-nil Swipe on success")
	}
	if swipeRepo.phaseBeforeAtCreate != domain.FSRSPhaseNew {
		t.Fatalf("PhaseBefore=%d, want FSRSPhaseNew (%d)", swipeRepo.phaseBeforeAtCreate, domain.FSRSPhaseNew)
	}
	if swipeRepo.phaseAfterAtCreate == nil || *swipeRepo.phaseAfterAtCreate == domain.FSRSPhaseNew {
		t.Fatalf("StateAfter.Phase must have advanced past FSRSPhaseNew, got %v", swipeRepo.phaseAfterAtCreate)
	}
	if !swipeRepo.created.DueBefore.Equal(swipeRepo.created.ReviewedAt) {
		t.Fatalf("DueBefore=%v, want the new card's review instant %v", swipeRepo.created.DueBefore, swipeRepo.created.ReviewedAt)
	}
	// A brand-new card starts at the NewFSRSStateForNewCard stability, so the
	// snapshot must read that value and not the post-rating stability.
	wantStability := domain.NewFSRSStateForNewCard(time.Now().UTC()).Stability
	if swipeRepo.stabilityBeforeAtCreate != wantStability {
		t.Fatalf("StabilityBefore=%v, want %v", swipeRepo.stabilityBeforeAtCreate, wantStability)
	}
	if swipeRepo.created.StateAfter.Stability == wantStability {
		t.Fatalf("StateAfter.Stability must have advanced past the new-card stability %v", wantStability)
	}
}

// TestSwipeUsecase_HandleSwipe_NonOwner_Unauthenticated verifies that
// HandleSwipe rejects a caller whose user ID does not match the cardgroup
// OwnerID.  authorizeCardgroupOrBadInput returns ucerr.ErrUnauthenticated for a
// non-owner, which the resolver layer translates into UNAUTHENTICATED on
// the wire.
func TestSwipeUsecase_HandleSwipe_NonOwner_Unauthenticated(t *testing.T) {
	t.Parallel()

	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-2"},
	}
	tx := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		&mockCardRepository{},
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
		service.NewFSRSScheduler(),
		tx,
		&mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}},
		newTestLogger(),
	)

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	assertUnauthenticated(t, err)
}

func TestSwipeUsecase_HandleSwipe_PropagatesUpsertError(t *testing.T) {
	t.Parallel()

	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-1"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	upsertErr := eris.New("storage: simulated upsert failure")
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID:  map[string]*domain.UserCardFSRS{},
		upsertErr: upsertErr,
	}
	tx := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		swipeRepo,
		service.NewFSRSScheduler(),
		tx,
		userFSRSRepo,
		newTestLogger(),
	)

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	if err == nil {
		t.Fatal("expected error from UpsertTx, got nil")
	}
	assertInternalChain(t, err, "storage: simulated upsert failure")
}

// TestSwipeUsecase_HandleSwipe_InvalidRating_ValidationVariant verifies that an
// invalid swipe rating surfaces as the outcome's Validation variant (not an
// error channel error) so the resolver maps it to the InputValidationError
// union member.
func TestSwipeUsecase_HandleSwipe_InvalidRating_ValidationVariant(t *testing.T) {
	t.Parallel()

	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	tx := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		&mockCardRepository{},
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
		service.NewFSRSScheduler(),
		tx,
		&mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}},
		newTestLogger(),
	)

	outcome, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      9999, // invalid
	})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Swipe != nil {
		t.Fatal("expected nil Swipe on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on invalid rating")
	}
	if outcome.Validation.Field != "rating" {
		t.Fatalf("expected Validation.Field=%q, got %q", "rating", outcome.Validation.Field)
	}
}

// TestSwipeUsecase_HandleSwipe_CardNotFound_ValidationVariant verifies that a
// card that cannot be found during the transaction surfaces as the outcome's
// Validation variant with field "cardId".
func TestSwipeUsecase_HandleSwipe_CardNotFound_ValidationVariant(t *testing.T) {
	t.Parallel()

	// FindByIDForUpdateTx returns (findResult, findErr); set findErr to ErrNotFound to
	// exercise the not-found branch inside the transaction closure.
	cardRepo := &mockCardRepository{
		findErr: repository.ErrNotFound,
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	tx := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		swipeRepo,
		service.NewFSRSScheduler(),
		tx,
		&mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}},
		newTestLogger(),
	)

	outcome, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-missing",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Swipe != nil {
		t.Fatal("expected nil Swipe on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on card-not-found")
	}
	if outcome.Validation.Field != "cardId" {
		t.Fatalf("expected Validation.Field=%q, got %q", "cardId", outcome.Validation.Field)
	}
}

// TestSwipeUsecase_HandleSwipe_CardgroupNotFound_ValidationVariant verifies that
// when authorizeCardgroupOrBadInput cannot find the cardgroup, HandleSwipe returns the
// Validation variant with field "cardgroupId" rather than an error channel error.
func TestSwipeUsecase_HandleSwipe_CardgroupNotFound_ValidationVariant(t *testing.T) {
	t.Parallel()

	// FindByID returns ErrNotFound → authorizeCardgroupOrBadInput returns
	// ucerr.NewValidationError("cardgroupId", "cardgroup not found") → liftValidationErr
	// promotes it to outcome.Validation.
	cardgroupRepo := &mockCardgroupRepoForCard{
		findErr: repository.ErrNotFound,
	}
	tx := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		&mockCardRepository{},
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
		service.NewFSRSScheduler(),
		tx,
		&mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}},
		newTestLogger(),
	)

	outcome, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-missing",
		Rating:      int(domain.RatingEasy),
	})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Swipe != nil {
		t.Fatal("expected nil Swipe on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on cardgroup-not-found")
	}
	if outcome.Validation.Field != "cardgroupId" {
		t.Fatalf("expected Validation.Field=%q, got %q", "cardgroupId", outcome.Validation.Field)
	}
	if outcome.Validation.Message != "cardgroup not found" {
		t.Fatalf("expected Validation.Message=%q, got %q", "cardgroup not found", outcome.Validation.Message)
	}
}

// TestSwipeUsecase_HandleSwipe_CardCrossCardgroup_ValidationVariant verifies that
// when the card's CardgroupID does not match the input CardgroupID, HandleSwipe
// returns the Validation variant with field "cardId" and message "card not found".
// This guards the cross-cardgroup mismatch branch at swipe.go (card.CardgroupID != in.CardgroupID).
func TestSwipeUsecase_HandleSwipe_CardCrossCardgroup_ValidationVariant(t *testing.T) {
	t.Parallel()

	// FindByIDForUpdateTx returns a card whose CardgroupID belongs to a different cardgroup.
	// The usecase must detect the mismatch and return a Validation outcome.
	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-other"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	tx := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
		service.NewFSRSScheduler(),
		tx,
		&mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}},
		newTestLogger(),
	)

	outcome, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Swipe != nil {
		t.Fatal("expected nil Swipe on cross-cardgroup mismatch")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on cross-cardgroup mismatch")
	}
	if outcome.Validation.Field != "cardId" {
		t.Fatalf("expected Validation.Field=%q, got %q", "cardId", outcome.Validation.Field)
	}
	if outcome.Validation.Message != "card not found" {
		t.Fatalf("expected Validation.Message=%q, got %q", "card not found", outcome.Validation.Message)
	}
}
