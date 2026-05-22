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
	recent     []*domain.SwipeRecord
	createErr  error
	listErr    error
	listUserID string
	listLimit  int
	created    *domain.SwipeRecord
}

func (m *mockSwipeRecordRepoForSwipe) CreateTx(_ context.Context, _ *gorm.DB, sr *domain.SwipeRecord) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = sr
	m.recent = append([]*domain.SwipeRecord{sr}, m.recent...)
	return nil
}

func (m *mockSwipeRecordRepoForSwipe) ListRecentByUser(_ context.Context, userID string, limit int) ([]*domain.SwipeRecord, error) {
	m.listUserID = userID
	m.listLimit = limit
	return m.recent, m.listErr
}

func TestSwipeUsecase_HandleSwipePerformanceMode(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		recent     []*domain.SwipeRecord
		wantMode   int
		wantReview int
	}{
		{
			name:       "less than twenty reviews uses default mode",
			recent:     performanceSwipes(base, 9, 9, 5),
			wantMode:   service.ModeDefault,
			wantReview: 19,
		},
		{
			name:       "sixty percent success with high difficulty becomes difficult",
			recent:     performanceSwipes(base, 11, 8, 8),
			wantMode:   service.ModeDifficult,
			wantReview: 20,
		},
		{
			name:       "ninety five percent success becomes in while",
			recent:     performanceSwipes(base, 18, 1, 5),
			wantMode:   service.ModeInWhile,
			wantReview: 20,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cardRepo := &mockCardRepository{
				findResult: &domain.Card{
					ID:          "card-1",
					CardgroupID: "cg-1",
				},
			}
			cardgroupRepo := &mockCardgroupRepoForCard{
				findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
			}
			swipeRepo := &mockSwipeRecordRepoForSwipe{recent: append([]*domain.SwipeRecord(nil), tc.recent...)}
			userFSRSRepo := &mockUserCardFSRSRepository{
				byCardID: map[string]*domain.UserCardFSRS{
					"card-1": domain.NewUserCardFSRSForNewCard("user-1", "card-1", base),
				},
			}
			tx, _ := fakeTxRunner()
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
				Mode:        int(domain.RatingEasy),
			})

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if outcome.Swipe == nil {
				t.Fatal("expected non-nil Swipe on success")
			}
			if outcome.Swipe.PerformanceMode != tc.wantMode {
				t.Fatalf("performance mode=%d, want %d; metrics=%+v", outcome.Swipe.PerformanceMode, tc.wantMode, outcome.Swipe.Metrics)
			}
			if outcome.Swipe.Metrics.ReviewCount != tc.wantReview {
				t.Fatalf("review count=%d, want %d", outcome.Swipe.Metrics.ReviewCount, tc.wantReview)
			}
			if swipeRepo.created == nil {
				t.Fatal("expected swipe record to be created before metrics are listed")
			}
			if swipeRepo.created.CardgroupID != "cg-1" {
				t.Fatalf("created swipe cardgroup=%q, want cg-1", swipeRepo.created.CardgroupID)
			}
			if swipeRepo.listUserID != "user-1" {
				t.Fatalf("ListRecentByUser user=%q, want user-1", swipeRepo.listUserID)
			}
			if swipeRepo.listLimit != swipePerformanceSampleLimit {
				t.Fatalf("ListRecentByUser limit=%d, want %d", swipeRepo.listLimit, swipePerformanceSampleLimit)
			}
		})
	}
}

func TestSwipeUsecase_HandleSwipeCreatesUserFSRSStateForFirstSwipe(t *testing.T) {
	t.Parallel()

	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: "cg-1",
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	userFSRSRepo := &mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}}
	tx, _ := fakeTxRunner()
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
		Mode:        int(domain.RatingEasy),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Swipe == nil {
		t.Fatal("expected non-nil Swipe on success")
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

// TestSwipeUsecase_HandleSwipe_NonOwner_Unauthenticated verifies that
// HandleSwipe rejects a caller whose user ID does not match the cardgroup
// OwnerID.  authorizeCardgroupOrBadInput returns ucerr.ErrUnauthenticated for a
// non-owner, which the resolver layer translates into UNAUTHENTICATED on
// the wire.
func TestSwipeUsecase_HandleSwipe_NonOwner_Unauthenticated(t *testing.T) {
	t.Parallel()

	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-2"},
	}
	tx, _ := fakeTxRunner()
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
		Mode:        int(domain.RatingEasy),
	})

	assertUnauthenticated(t, err)
}

func performanceSwipes(now time.Time, successes, failures int, difficulty float64) []*domain.SwipeRecord {
	swipes := make([]*domain.SwipeRecord, 0, successes+failures)
	for i := range successes {
		swipes = append(swipes, performanceSwipe(domain.RatingEasy, now.Add(-time.Duration(i+1)*time.Hour), difficulty))
	}
	for i := range failures {
		swipes = append(swipes, performanceSwipe(domain.RatingAgain, now.Add(-time.Duration(successes+i+1)*time.Hour), difficulty))
	}
	return swipes
}

func performanceSwipe(rating domain.Rating, reviewedAt time.Time, difficulty float64) *domain.SwipeRecord {
	return &domain.SwipeRecord{
		ID:         reviewedAt.Format("20060102150405"),
		UserID:     "user-1",
		CardID:     "card-1",
		Rating:     rating,
		ReviewedAt: reviewedAt,
		StateAfter: domain.FSRSState{
			Difficulty:    difficulty,
			ElapsedDays:   1,
			ScheduledDays: 1,
			State:         domain.FSRSStateReview,
		},
	}
}

func TestSwipeUsecase_HandleSwipe_PropagatesUpsertError(t *testing.T) {
	t.Parallel()

	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: "cg-1",
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	upsertErr := eris.New("storage: simulated upsert failure")
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID:  map[string]*domain.UserCardFSRS{},
		upsertErr: upsertErr,
	}
	tx, _ := fakeTxRunner()
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
		Mode:        int(domain.RatingEasy),
	})

	if err == nil {
		t.Fatal("expected error from UpsertTx, got nil")
	}
	assertInternalChain(t, err, "storage: simulated upsert failure")
}

// TestSwipeUsecase_HandleSwipe_InvalidMode_ValidationVariant verifies that an
// invalid swipe mode surfaces as the outcome's Validation variant (not an
// error channel error) so the resolver maps it to the InputValidationError
// union member.
func TestSwipeUsecase_HandleSwipe_InvalidMode_ValidationVariant(t *testing.T) {
	t.Parallel()

	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
	}
	tx, _ := fakeTxRunner()
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
		Mode:        9999, // invalid
	})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Swipe != nil {
		t.Fatal("expected nil Swipe on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on invalid mode")
	}
	if outcome.Validation.Field != "mode" {
		t.Fatalf("expected Validation.Field=%q, got %q", "mode", outcome.Validation.Field)
	}
}

// TestSwipeUsecase_HandleSwipe_CardNotFound_ValidationVariant verifies that a
// card that cannot be found during the transaction surfaces as the outcome's
// Validation variant with field "cardId".
func TestSwipeUsecase_HandleSwipe_CardNotFound_ValidationVariant(t *testing.T) {
	t.Parallel()

	// FindByIDTx returns (findResult, findErr); set findErr to ErrNotFound to
	// exercise the not-found branch inside the transaction closure.
	cardRepo := &mockCardRepository{
		findErr: repository.ErrNotFound,
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{}
	tx, _ := fakeTxRunner()
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
		Mode:        int(domain.RatingEasy),
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
	tx, _ := fakeTxRunner()
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
		Mode:        int(domain.RatingEasy),
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

	// FindByIDTx returns a card whose CardgroupID belongs to a different cardgroup.
	// The usecase must detect the mismatch and return a Validation outcome.
	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: "cg-other",
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
	}
	tx, _ := fakeTxRunner()
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
		Mode:        int(domain.RatingEasy),
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
