package repository

// White-box tests for classifyCardFKError. The function is unexported so the
// tests must live in the same package. All assertions use fabricated
// *pgconn.PgError values — no live DB is required. Mirrors
// master_card_classify_fk_test.go.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyCardFKError_CardgroupConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "cards_cardgroup_id_fkey",
	}
	got := classifyCardFKError(pgErr)
	if !errors.Is(got, ErrCardCardgroupNotFound) {
		t.Fatalf("expected ErrCardCardgroupNotFound, got %v", got)
	}
	// Must also satisfy the legacy sentinel.
	if !errors.Is(got, ErrNotFound) {
		t.Fatalf("expected errors.Is(_, ErrNotFound) true, got %v", got)
	}
}

func TestClassifyCardFKError_SentinelJoinsErrNotFound(t *testing.T) {
	t.Parallel()
	// The sentinel itself carries the ErrNotFound join, independent of any
	// classifier call, so callers matching the general sentinel keep working.
	if !errors.Is(ErrCardCardgroupNotFound, ErrNotFound) {
		t.Fatal("ErrCardCardgroupNotFound must match ErrNotFound")
	}
}

func TestClassifyCardFKError_UnknownConstraint(t *testing.T) {
	t.Parallel()
	// A 23503 on a constraint that does not name cardgroup_id must return nil
	// so the caller falls through to eris.Wrap.
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "cards_other_fkey",
	}
	if got := classifyCardFKError(pgErr); got != nil {
		t.Fatalf("expected nil for unknown FK constraint, got %v", got)
	}
}

func TestClassifyCardFKError_NotAFKViolation(t *testing.T) {
	t.Parallel()
	// A non-23503 Postgres error must not be classified.
	if got := classifyCardFKError(&pgconn.PgError{Code: "23505", ConstraintName: "uq_cards_cardgroup_front"}); got != nil {
		t.Fatalf("expected nil for non-FK error, got %v", got)
	}
}

func TestClassifyCardFKError_NonPgError(t *testing.T) {
	t.Parallel()
	if got := classifyCardFKError(errors.New("some error")); got != nil {
		t.Fatalf("expected nil for non-pg error, got %v", got)
	}
}

func TestClassifyCardFKError_NilError(t *testing.T) {
	t.Parallel()
	if got := classifyCardFKError(nil); got != nil {
		t.Fatalf("expected nil for nil error, got %v", got)
	}
}
