package repository

// White-box tests for classifyMasterCardDuplicateFront. The function is
// unexported so the tests must live in the same package. All assertions use
// fabricated *pgconn.PgError values — no live DB is required. Mirrors
// master_card_classify_fk_test.go.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyMasterCardDuplicateFront_MatchingConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "uq_master_cards_cg_front",
	}
	got := classifyMasterCardDuplicateFront(pgErr)
	if !errors.Is(got, ErrCardDuplicateFront) {
		t.Fatalf("expected ErrCardDuplicateFront, got %v", got)
	}
}

func TestClassifyMasterCardDuplicateFront_OtherUniqueConstraint(t *testing.T) {
	t.Parallel()
	// A 23505 on a constraint other than the (master_cardgroup_id, front) index
	// must return nil so the caller falls through to eris.Wrap.
	pgErr := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "master_cards_pkey",
	}
	if got := classifyMasterCardDuplicateFront(pgErr); got != nil {
		t.Fatalf("expected nil for a non-front unique violation, got %v", got)
	}
}

func TestClassifyMasterCardDuplicateFront_NotAUniqueViolation(t *testing.T) {
	t.Parallel()
	// A non-23505 Postgres error must not be classified even when the constraint
	// name matches.
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "uq_master_cards_cg_front",
	}
	if got := classifyMasterCardDuplicateFront(pgErr); got != nil {
		t.Fatalf("expected nil for non-23505 error, got %v", got)
	}
}

func TestClassifyMasterCardDuplicateFront_NonPgError(t *testing.T) {
	t.Parallel()
	if got := classifyMasterCardDuplicateFront(errors.New("some error")); got != nil {
		t.Fatalf("expected nil for non-pg error, got %v", got)
	}
}

func TestClassifyMasterCardDuplicateFront_NilError(t *testing.T) {
	t.Parallel()
	if got := classifyMasterCardDuplicateFront(nil); got != nil {
		t.Fatalf("expected nil for nil error, got %v", got)
	}
}
