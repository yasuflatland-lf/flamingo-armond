package repository

// White-box tests for classifyCardgroupOwnerFKError. The function is unexported
// so the tests must live in the same package. All assertions use fabricated
// *pgconn.PgError values — no live DB is required. Mirrors
// master_card_classify_fk_test.go.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyCardgroupOwnerFKError_NotAFKViolation(t *testing.T) {
	t.Parallel()
	// A non-23503 Postgres error must not be classified.
	if got := classifyCardgroupOwnerFKError(&pgconn.PgError{Code: "23505"}); got != nil {
		t.Fatalf("expected nil for non-FK error, got %v", got)
	}
}

func TestClassifyCardgroupOwnerFKError_NonPgError(t *testing.T) {
	t.Parallel()
	if got := classifyCardgroupOwnerFKError(errors.New("some error")); got != nil {
		t.Fatalf("expected nil for non-pg error, got %v", got)
	}
}

func TestClassifyCardgroupOwnerFKError_OwnerConstraint(t *testing.T) {
	t.Parallel()
	// The constraint name Postgres generates for
	// cardgroups.owner_id REFERENCES public.users(id).
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "cardgroups_owner_id_fkey",
	}
	got := classifyCardgroupOwnerFKError(pgErr)
	if !errors.Is(got, ErrCardgroupOwnerNotFound) {
		t.Fatalf("expected ErrCardgroupOwnerNotFound, got %v", got)
	}
	// The sentinel is deliberately standalone: a missing owner is not a missing
	// cardgroup, so callers branching on ErrNotFound must not match it.
	if errors.Is(got, ErrNotFound) {
		t.Fatalf("ErrCardgroupOwnerNotFound must not satisfy ErrNotFound, got %v", got)
	}
}

func TestClassifyCardgroupOwnerFKError_UnknownConstraint(t *testing.T) {
	t.Parallel()
	// A 23503 on a constraint that does not name owner_id must return nil so the
	// caller falls through to eris.Wrap.
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "cardgroups_other_fkey",
	}
	if got := classifyCardgroupOwnerFKError(pgErr); got != nil {
		t.Fatalf("expected nil for unknown FK constraint, got %v", got)
	}
}

func TestClassifyCardgroupOwnerFKError_NilError(t *testing.T) {
	t.Parallel()
	if got := classifyCardgroupOwnerFKError(nil); got != nil {
		t.Fatalf("expected nil for nil error, got %v", got)
	}
}
