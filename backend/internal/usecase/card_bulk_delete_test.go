package usecase

import (
	"context"
	"fmt"
	"testing"

	"gorm.io/gorm"
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
	cardRepo := &mockCardRepository{
		deleteByIDsResult: 3,
	}
	cgRepo := &mockCardgroupRepoForCard{}
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

// TestCardUsecase_BulkDelete_SilentlySkipsForeign verifies that mixing own and
// foreign ids does NOT cause the call to fail. The SQL subselect in
// DeleteByIDsTx handles ownership filtering; the usecase passes all ids through
// and returns the count of rows actually deleted (own ones only).
func TestCardUsecase_BulkDelete_SilentlySkipsForeign(t *testing.T) {
	t.Parallel()
	// DeleteByIDsTx receives all ids but the SQL subselect filters to own cards
	// only, returning 1 (the count of own cards deleted).
	cardRepo := &mockCardRepository{deleteByIDsResult: 1}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, calls := fakeTxRunner()
	uc := &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx}

	n, err := uc.BulkDelete(authedCtx("u1"), []string{"c1", "foreign"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 deleted (own card only), got %d", n)
	}
	if *calls != 1 {
		t.Fatalf("expected 1 tx invocation, got %d", *calls)
	}
	// Both ids must be forwarded to the repository so the SQL subselect can
	// decide which ones to delete.
	if len(cardRepo.capturedDeleteIDs) != 2 {
		t.Fatalf("expected 2 ids forwarded to DeleteByIDsTx, got %d", len(cardRepo.capturedDeleteIDs))
	}
}

// TestCardUsecase_BulkDelete_RejectsTooManyIDs verifies that supplying more
// than maxBulkDelete ids returns BAD_USER_INPUT on the "ids" field.
func TestCardUsecase_BulkDelete_RejectsTooManyIDs(t *testing.T) {
	t.Parallel()
	ids := make([]string, maxBulkDelete+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("c%d", i)
	}
	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, calls := fakeTxRunner()
	uc := &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx}

	_, err := uc.BulkDelete(authedCtx("u1"), ids)
	assertGQLErr(t, err, "BAD_USER_INPUT", "ids")
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations, got %d", *calls)
	}
}
