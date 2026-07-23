package usecase

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
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
	uc := &cardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx, logger: newTestLogger()}

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
	uc := &cardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx, logger: newTestLogger()}

	_, err := uc.BulkDelete(anonCtx(), []string{"c1", "c2"})
	assertUnauthenticated(t, err)
	if cardRepo.deleteByIDsCalls != 0 {
		t.Fatalf("expected 0 DeleteByIDsTx invocations on anonymous, got %d", cardRepo.deleteByIDsCalls)
	}
}

func TestCardUsecase_BulkDelete_AnonymousWithEmptyIDs(t *testing.T) {
	// Auth check fires before the empty-ids short-circuit.
	t.Parallel()
	uc := &cardUsecase{cardRepo: &mockCardRepository{}, cardgroupRepo: &mockCardgroupRepoForCard{}, logger: newTestLogger()}
	_, err := uc.BulkDelete(anonCtx(), nil)
	assertUnauthenticated(t, err)
}

func TestCardUsecase_BulkDelete_AllOwn(t *testing.T) {
	t.Parallel()
	cardRepo := &mockCardRepository{
		deleteByIDsResult: 3,
	}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, calls := fakeTxRunner()
	uc := &cardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx, logger: newTestLogger()}

	n, err := uc.BulkDelete(authedCtx("u1"), []string{uuid.NewString(), uuid.NewString(), uuid.NewString()})
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
	uc := &cardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx, logger: newTestLogger()}

	n, err := uc.BulkDelete(authedCtx("u1"), []string{uuid.NewString(), uuid.NewString()})
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

// TestCardUsecase_BulkDelete_PartialMatchSucceeds verifies that when the SQL
// subselect filters out foreign-owned ids (deleted < len(ids)), BulkDelete still
// succeeds and returns the actual deleted count. The partial-match INFO log line
// emitted via u.logger.LogAttrs is observed indirectly: the test asserts the return
// value, and the SilentlySkipsForeign test pins behavior on both sides of the log.
func TestCardUsecase_BulkDelete_PartialMatchSucceeds(t *testing.T) {
	t.Parallel()
	// Repository reports 2 deleted out of 5 requested — simulates the SQL
	// subselect filtering out 3 foreign-owned cards.
	cardRepo := &mockCardRepository{deleteByIDsResult: 2}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, calls := fakeTxRunner()
	uc := &cardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx, logger: newTestLogger()}

	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	n, err := uc.BulkDelete(authedCtx("u1"), ids)
	if err != nil {
		t.Fatalf("unexpected error on partial match: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}
	if *calls != 1 {
		t.Fatalf("expected 1 tx invocation, got %d", *calls)
	}
}

// TestCardUsecase_BulkDelete_DropsMalformedIDs verifies malformed (non-UUID)
// ids are dropped before the SQL runs: the repo receives only the parseable ids
// in order, and the partial-match log still compares against the ORIGINAL
// request count so dropped malformed ids surface in the log exactly like
// foreign-owned skips. The id column is uuid-typed, so a malformed id cannot
// match any row; aborting the whole batch with SQLSTATE 22P02 would contradict
// the documented silent-skip semantics.
func TestCardUsecase_BulkDelete_DropsMalformedIDs(t *testing.T) {
	t.Parallel()
	cardRepo := &mockCardRepository{deleteByIDsResult: 2}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, calls := fakeTxRunner()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	uc := &cardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx, logger: logger}

	valid1 := uuid.NewString()
	valid2 := uuid.NewString()
	n, err := uc.BulkDelete(authedCtx("u1"), []string{valid1, "not-a-uuid", valid2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 deleted, got %d", n)
	}
	if *calls != 1 {
		t.Fatalf("expected 1 tx invocation, got %d", *calls)
	}
	if len(cardRepo.capturedDeleteIDs) != 2 ||
		cardRepo.capturedDeleteIDs[0] != valid1 || cardRepo.capturedDeleteIDs[1] != valid2 {
		t.Fatalf("expected repo to receive only the valid ids in order, got %v", cardRepo.capturedDeleteIDs)
	}
	// deleted (2) < requested (3): the partial-match log fires against the
	// original count, so the dropped malformed id is observable.
	logged := buf.String()
	if !strings.Contains(logged, "bulk delete: partial match") || !strings.Contains(logged, `"requested":3`) {
		t.Fatalf("expected partial-match log against the original count of 3, got %q", logged)
	}
}

// TestCardUsecase_BulkDelete_AllMalformedIDs verifies an all-malformed batch
// short-circuits to (0, nil) without opening a transaction or touching the
// repository.
func TestCardUsecase_BulkDelete_AllMalformedIDs(t *testing.T) {
	t.Parallel()
	cardRepo := &mockCardRepository{}
	cgRepo := &mockCardgroupRepoForCard{}
	tx, calls := fakeTxRunner()
	uc := &cardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx, logger: newTestLogger()}

	n, err := uc.BulkDelete(authedCtx("u1"), []string{"not-a-uuid", "also-junk"})
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
	uc := &cardUsecase{cardRepo: cardRepo, cardgroupRepo: cgRepo, tx: tx, logger: newTestLogger()}

	_, err := uc.BulkDelete(authedCtx("u1"), ids)
	assertValidationError(t, err, "ids", "")
	if *calls != 0 {
		t.Fatalf("expected 0 tx invocations, got %d", *calls)
	}
}
