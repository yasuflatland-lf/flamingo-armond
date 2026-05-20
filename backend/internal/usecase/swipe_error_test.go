package usecase

import (
	"errors"
	"testing"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/domain/service"
)

// TestSwipeUsecase_HandleSwipe_FindCardByIDError_PinsChain verifies that an
// infrastructure error from FindByIDTx is wrapped with the canonical
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
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
	}
	tx, _ := fakeTxRunner()
	uc := NewSwipeUsecaseWithTx(
		cardRepo,
		cardgroupRepo,
		&mockSwipeRecordRepoForSwipe{},
		service.NewFSRSScheduler(),
		10,
		tx,
		&mockUserCardFSRSRepository{byCardID: map[string]*domain.UserCardFSRS{}},
		newTestLogger(),
	)

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Mode:        int(domain.RatingEasy),
	})

	assertInternalChain(t, err, "usecase: swipe: find card by id")
	if !errors.Is(err, infraErr) {
		t.Fatalf("error chain should preserve injected root sentinel; got: %v", err)
	}
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
			CardgroupID: "cg-1",
		},
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
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
		10,
		tx,
		userFSRSRepo,
		newTestLogger(),
	)

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Mode:        int(domain.RatingEasy),
	})

	assertInternalChain(t, err, "usecase: swipe: find user-card fsrs")
	if !errors.Is(err, infraErr) {
		t.Fatalf("error chain should preserve injected root sentinel; got: %v", err)
	}
}

// TestSwipeUsecase_HandleSwipe_FindDueCardsError_PinsChain verifies that an
// infrastructure error from FindDueCardsForUserTx (the final step that fetches
// the next batch of due cards) is wrapped with the canonical
// "usecase: swipe: find due cards" prefix.  This is the central new path
// introduced by the ordering-policy refactor.
func TestSwipeUsecase_HandleSwipe_FindDueCardsError_PinsChain(t *testing.T) {
	t.Parallel()

	infraErr := eris.New("storage: simulated find-due-cards infra failure")
	cardRepo := &mockCardRepository{
		findResult: &domain.Card{
			ID:          "card-1",
			CardgroupID: "cg-1",
		},
		findDueErr: infraErr,
	}
	cardgroupRepo := &mockCardgroupRepoForCard{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "user-1"},
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
		10,
		tx,
		userFSRSRepo,
		newTestLogger(),
	)

	_, err := uc.HandleSwipe(authedCtx("user-1"), HandleSwipeInput{
		CardID:      "card-1",
		CardgroupID: "cg-1",
		Mode:        int(domain.RatingEasy),
	})

	assertInternalChain(t, err, "usecase: swipe: find due cards")
	if !errors.Is(err, infraErr) {
		t.Fatalf("error chain should preserve injected root sentinel; got: %v", err)
	}
}
