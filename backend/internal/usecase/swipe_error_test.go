package usecase

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/domain/service"
)

// TestSwipeUsecase_HandleSwipe_FindCardByIDError_PinsChain verifies that an
// infrastructure error from FindByIDForUpdateTx is wrapped with the canonical
// "usecase: swipe: find card by id" prefix so the error_chain log attribute
// points at the correct operation.  repository.ErrNotFound is NOT used here;
// that sentinel travels to the Validation outcome variant and is covered by
// TestSwipeUsecase_HandleSwipe_CardNotFound_ValidationVariant.
func TestSwipeUsecase_HandleSwipe_FindCardByIDError_PinsChain(t *testing.T) {
	t.Parallel()

	infraErr := eris.New("storage: simulated find-card infra failure")
	cardRepo := &mockCardRepository{
		findErr: infraErr,
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
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

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	assertInternalChain(t, err, "usecase: swipe: find card by id")
	require.ErrorIs(t, err, infraErr, "error chain must preserve injected root sentinel")
}

// TestSwipeUsecase_HandleSwipe_FindUserCardFSRSError_PinsChain verifies that an
// infrastructure error from FindByUserAndCardIDsTx is wrapped with the canonical
// "usecase: swipe: find user-card fsrs" prefix.
func TestSwipeUsecase_HandleSwipe_FindUserCardFSRSError_PinsChain(t *testing.T) {
	t.Parallel()

	infraErr := eris.New("storage: simulated find-user-card-fsrs infra failure")
	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-1"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
		findErr:  infraErr,
	}
	tx, _ := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
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

	assertInternalChain(t, err, "usecase: swipe: find user-card fsrs")
	require.ErrorIs(t, err, infraErr, "error chain must preserve injected root sentinel")
}

// TestSwipeUsecase_HandleSwipe_ApplyRatingError_PinsChain verifies that an
// infrastructure error from the rating application step is wrapped with the
// canonical "usecase: swipe: apply rating" prefix.
func TestSwipeUsecase_HandleSwipe_ApplyRatingError_PinsChain(t *testing.T) {
	t.Parallel()

	infraErr := eris.New("storage: simulated apply-rating infra failure")
	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-1"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
	}
	tx, _ := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
		service.NewFSRSScheduler(),
		tx,
		userFSRSRepo,
		newTestLogger(),
	)
	uc.(*swipeUsecase).applyRating = func(_ *domain.UserCardFSRS, _ domain.FSRSScheduler, _ domain.Rating, _ time.Time) error {
		return infraErr
	}

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	assertInternalChain(t, err, "usecase: swipe: apply rating")
	require.ErrorIs(t, err, infraErr, "error chain must preserve injected root sentinel")
}

// TestSwipeUsecase_HandleSwipe_NewSwipeRecordError_PinsChain verifies that a
// failure while constructing the denormalized swipe snapshot is wrapped with
// the canonical "usecase: swipe: new swipe record" prefix.
func TestSwipeUsecase_HandleSwipe_NewSwipeRecordError_PinsChain(t *testing.T) {
	t.Parallel()

	infraErr := eris.New("storage: simulated new-swipe-record infra failure")
	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-1"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
	}
	tx, _ := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
		service.NewFSRSScheduler(),
		tx,
		userFSRSRepo,
		newTestLogger(),
	)
	uc.(*swipeUsecase).newSwipeRecord = func(_ domain.UserID, _ string, _ domain.CardgroupID, _ domain.Rating, _ time.Time, _, _ domain.FSRSState) (*domain.SwipeRecord, error) {
		return nil, infraErr
	}

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	assertInternalChain(t, err, "usecase: swipe: new swipe record")
	require.ErrorIs(t, err, infraErr, "error chain must preserve injected root sentinel")
}

// TestSwipeUsecase_HandleSwipe_InsertSwipeRecordError_PinsChain verifies that
// infrastructure errors from the final insert step preserve the canonical
// "usecase: swipe: insert swipe record" prefix.
func TestSwipeUsecase_HandleSwipe_InsertSwipeRecordError_PinsChain(t *testing.T) {
	t.Parallel()

	infraErr := eris.New("storage: simulated insert-swipe-record infra failure")
	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-1"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{
		createErr: infraErr,
	}
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
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
		Rating:      int(domain.RatingEasy),
	})

	assertInternalChain(t, err, "usecase: swipe: insert swipe record")
	require.ErrorIs(t, err, infraErr, "error chain must preserve injected root sentinel")
}

// TestSwipeUsecase_HandleSwipe_FindCardByID_PropagatesCancelled verifies that
// context.Canceled returned from FindByIDForUpdateTx passes through unwrapped so the
// caller can distinguish a cancellation from an infrastructure failure.
func TestSwipeUsecase_HandleSwipe_FindCardByID_PropagatesCancelled(t *testing.T) {
	t.Parallel()

	cardRepo := &mockCardRepository{findErr: context.Canceled}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
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

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestSwipeUsecase_HandleSwipe_FindUserCardFSRS_PropagatesCancelled verifies
// that context.Canceled returned from FindByUserAndCardIDsTx passes through
// unwrapped after FindByIDForUpdateTx succeeds.
func TestSwipeUsecase_HandleSwipe_FindUserCardFSRS_PropagatesCancelled(t *testing.T) {
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
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
		findErr:  context.Canceled,
	}
	tx, _ := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
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

	assertCancelled(t, err)
}

// TestSwipeUsecase_HandleSwipe_UpsertUserCardFSRS_PropagatesCancelled verifies
// that context.Canceled returned from UpsertTx passes through unwrapped after
// the preceding find steps succeed.
func TestSwipeUsecase_HandleSwipe_UpsertUserCardFSRS_PropagatesCancelled(t *testing.T) {
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
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID:  map[string]*domain.UserCardFSRS{},
		upsertErr: context.Canceled,
	}
	tx, _ := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
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

	assertCancelled(t, err)
}

// TestSwipeUsecase_HandleSwipe_InsertSwipeRecord_PropagatesCancelled verifies
// that context.Canceled returned from CreateTx passes through unwrapped after
// the upsert step succeeds.
func TestSwipeUsecase_HandleSwipe_InsertSwipeRecord_PropagatesCancelled(t *testing.T) {
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
	swipeRepo := &mockSwipeRecordRepoForSwipe{
		createErr: context.Canceled,
	}
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
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
		Rating:      int(domain.RatingEasy),
	})

	assertCancelled(t, err)
}

// TestSwipeUsecase_HandleSwipe_ListRecentSwipes_DegradesToSuccess verifies that
// an infrastructure error from the post-commit ListRecentByUser read does NOT
// fail the mutation: the swipe row and the FSRS row are already durable, so the
// outcome carries the neutral empty-window snapshot (service.ModeDefault) and
// the failure is only logged with the canonical
// "usecase: swipe: list recent swipes" chain prefix. Reporting a committed
// swipe as failed would make the client re-queue the card and review it twice.
func TestSwipeUsecase_HandleSwipe_ListRecentSwipes_DegradesToSuccess(t *testing.T) {
	t.Parallel()

	infraErr := eris.New("storage: simulated list-recent infra failure")
	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: domain.CardgroupID("cg-1"),
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "user-1"},
	}
	swipeRepo := &mockSwipeRecordRepoForSwipe{
		listErr: infraErr,
	}
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
	}
	tx, _ := fakeTxRunner()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		swipeRepo,
		service.NewFSRSScheduler(),
		tx,
		userFSRSRepo,
		logger,
	)

	out, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Rating:      int(domain.RatingEasy),
	})

	require.NoError(t, err, "a committed swipe must not surface the post-commit read failure")
	require.Nil(t, out.Validation)
	require.NotNil(t, out.Swipe)
	require.Equal(t, int(service.ModeDefault), out.Swipe.PerformanceMode)
	require.Equal(t, 0, out.Swipe.Metrics.ReviewCount, "degraded snapshot carries no reviews")

	// Both writes are still durable: the FSRS row was upserted and the swipe
	// record was inserted inside the committed transaction.
	require.NotNil(t, userFSRSRepo.upserted, "FSRS row must still be committed")
	require.NotNil(t, swipeRepo.created, "swipe record must still be committed")

	logged := buf.String()
	require.Contains(t, logged, "swipe committed but recent-swipe read failed")
	require.Contains(t, logged, "usecase: swipe: list recent swipes")
	require.Contains(t, logged, "simulated list-recent infra failure")
}

// TestSwipeUsecase_HandleSwipe_ListRecentSwipes_PropagatesCancelled verifies
// that context.Canceled returned from ListRecentByUser passes through unwrapped
// after the transaction commits successfully. Cancellation is the deliberate
// exception to the degrade-to-success rule: the caller is being torn down and
// has nothing to report to.
func TestSwipeUsecase_HandleSwipe_ListRecentSwipes_PropagatesCancelled(t *testing.T) {
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
	swipeRepo := &mockSwipeRecordRepoForSwipe{
		listErr: context.Canceled,
	}
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
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
		Rating:      int(domain.RatingEasy),
	})

	assertCancelled(t, err)
	require.Equal(t, context.Canceled, err, "expected unwrapped context.Canceled, got %v", err)
}

// TestSwipeUsecase_HandleSwipe_ListRecentSwipes_PropagatesDeadlineExceeded
// verifies that context.DeadlineExceeded returned from ListRecentByUser passes
// through unwrapped after the transaction commits successfully — the same
// deliberate exception to the degrade-to-success rule as the cancelled case.
func TestSwipeUsecase_HandleSwipe_ListRecentSwipes_PropagatesDeadlineExceeded(t *testing.T) {
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
	swipeRepo := &mockSwipeRecordRepoForSwipe{
		listErr: context.DeadlineExceeded,
	}
	userFSRSRepo := &mockUserCardFSRSRepository{
		byCardID: map[string]*domain.UserCardFSRS{},
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
		Rating:      int(domain.RatingEasy),
	})

	assertCancelled(t, err)
	require.Equal(t, context.DeadlineExceeded, err, "expected unwrapped context.DeadlineExceeded, got %v", err)
}
