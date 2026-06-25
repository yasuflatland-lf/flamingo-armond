package repository

// White-box tests for classifyMasterCardFKError. The function is unexported so
// the tests must live in the same package. All assertions use fabricated
// *pgconn.PgError values — no live DB is required. Mirrors role_classify_fk_test.go.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyMasterCardFKError_NotAFKViolation(t *testing.T) {
	t.Parallel()
	// A non-23503 Postgres error must not be classified.
	if got := classifyMasterCardFKError(&pgconn.PgError{Code: "23505"}); got != nil {
		t.Fatalf("expected nil for non-FK error, got %v", got)
	}
}

func TestClassifyMasterCardFKError_NonPgError(t *testing.T) {
	t.Parallel()
	if got := classifyMasterCardFKError(errors.New("some error")); got != nil {
		t.Fatalf("expected nil for non-pg error, got %v", got)
	}
}

func TestClassifyMasterCardFKError_MasterCardgroupConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "master_cards_master_cardgroup_id_fkey",
	}
	got := classifyMasterCardFKError(pgErr)
	if !errors.Is(got, ErrMasterCardgroupNotFound) {
		t.Fatalf("expected ErrMasterCardgroupNotFound, got %v", got)
	}
	// Must also satisfy the legacy sentinel.
	if !errors.Is(got, ErrNotFound) {
		t.Fatalf("expected errors.Is(_, ErrNotFound) true, got %v", got)
	}
}

func TestClassifyMasterCardFKError_UnknownConstraint(t *testing.T) {
	t.Parallel()
	// A 23503 on a constraint that does not name master_cardgroup_id must return
	// nil so the caller falls through to eris.Wrap.
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "master_cards_other_fkey",
	}
	if got := classifyMasterCardFKError(pgErr); got != nil {
		t.Fatalf("expected nil for unknown FK constraint, got %v", got)
	}
}

func TestClassifyMasterCardFKError_NilError(t *testing.T) {
	t.Parallel()
	if got := classifyMasterCardFKError(nil); got != nil {
		t.Fatalf("expected nil for nil error, got %v", got)
	}
}
