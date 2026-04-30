package usecase

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"backend/internal/domain"
)

// fakeTxRunner returns a txRunner that invokes fn with a nil *gorm.DB. The
// repository under test is the mock, which ignores tx anyway, so this is
// sufficient to exercise BulkDelete without a real database.
func fakeTxRunner() (txRunner, *int) {
	calls := 0
	return func(_ context.Context, fn func(tx *gorm.DB) error) error {
		calls++
		return fn(nil)
	}, &calls
}

func TestCardUsecase_BulkDelete_EmptyIDs(t *testing.T) {
	t.Parallel()
	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, calls := fakeTxRunner()
	uc := &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx}

	n, err := uc.BulkDelete(authedCtx("u1"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 deleted, got %d", n)
	}
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations, got %d", *calls)
	}
	if cardRepo.deleteByIDsCalls != 0 {
		t.Fatalf("expected 0 DeleteByIDsTx invocations, got %d", cardRepo.deleteByIDsCalls)
	}
}

func TestCardUsecase_BulkDelete_Anonymous(t *testing.T) {
	t.Parallel()
	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, _ := fakeTxRunner()
	uc := &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx}

	_, err := uc.BulkDelete(anonCtx(), []string{"c1", "c2"})
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if cardRepo.deleteByIDsCalls != 0 {
		t.Fatalf("expected 0 DeleteByIDsTx invocations on anonymous, got %d", cardRepo.deleteByIDsCalls)
	}
}

func TestCardUsecase_BulkDelete_AnonymousWithEmptyIDs(t *testing.T) {
	// Auth check fires before the empty-ids short-circuit.
	t.Parallel()
	uc := &CardUsecase{cardRepo: &mockCardRepository{}, cardgroupRepo: &mockCardgroupRepoForCard{}}
	_, err := uc.BulkDelete(anonCtx(), nil)
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardUsecase_BulkDelete_AllOwn(t *testing.T) {
	t.Parallel()
	cards := map[string]*domain.Card{
		"c1": {ID: "c1", CardgroupID: "cg1"},
		"c2": {ID: "c2", CardgroupID: "cg1"},
		"c3": {ID: "c3", CardgroupID: "cg2"},
	}
	cgs := map[string]*domain.Cardgroup{
		"cg1": {ID: "cg1", OwnerID: "u1"},
		"cg2": {ID: "cg2", OwnerID: "u1"},
	}
	cardRepo := &mockCardRepository{
		findByIDsResult:   cards,
		deleteByIDsResult: 3,
	}
	cgRepo := &mockCardgroupRepoForCard{findByIDsResult: cgs}
	tx, calls := fakeTxRunner()
	uc := &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx}

	n, err := uc.BulkDelete(authedCtx("u1"), []string{"c1", "c2", "c3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 deleted, got %d", n)
	}
	if *calls != 1 {
		t.Fatalf("expected 1 tx invocation, got %d", *calls)
	}
	if cardRepo.capturedDeleteOwn != "u1" {
		t.Fatalf("expected DeleteByIDsTx to receive owner=u1, got %q", cardRepo.capturedDeleteOwn)
	}
	if len(cardRepo.capturedDeleteIDs) != 3 {
		t.Fatalf("expected 3 ids passed to DeleteByIDsTx, got %d", len(cardRepo.capturedDeleteIDs))
	}
}

func TestCardUsecase_BulkDelete_MixOwnAndForeign(t *testing.T) {
	t.Parallel()
	cards := map[string]*domain.Card{
		"c1":      {ID: "c1", CardgroupID: "cg1"},
		"foreign": {ID: "foreign", CardgroupID: "cg-other"},
	}
	cgs := map[string]*domain.Cardgroup{
		"cg1":      {ID: "cg1", OwnerID: "u1"},
		"cg-other": {ID: "cg-other", OwnerID: "u-other"},
	}
	cardRepo := &mockCardRepository{findByIDsResult: cards}
	cgRepo := &mockCardgroupRepoForCard{findByIDsResult: cgs}
	tx, calls := fakeTxRunner()
	uc := &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx}

	_, err := uc.BulkDelete(authedCtx("u1"), []string{"c1", "foreign"})
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if *calls != 0 {
		t.Fatalf("expected DeleteByIDsTx not to run when any id is foreign, got %d tx invocations", *calls)
	}
	if cardRepo.deleteByIDsCalls != 0 {
		t.Fatalf("expected 0 DeleteByIDsTx invocations, got %d", cardRepo.deleteByIDsCalls)
	}
}

func TestCardUsecase_BulkDelete_FindByIDsErrorBecomesInternal(t *testing.T) {
	t.Parallel()
	cardRepo := &mockCardRepository{findByIDsErr: errors.New("db died")}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, _ := fakeTxRunner()
	uc := &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx}

	_, err := uc.BulkDelete(authedCtx("u1"), []string{"c1"})
	assertGQLErr(t, err, "INTERNAL", "")
}
