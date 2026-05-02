package repository

// White-box tests for classifyUserCardgroupFKError. The function is unexported
// so the tests must live in the same package. All assertions use fabricated
// *pgconn.PgError values — no live DB is required.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestClassifyUserCardgroupFKError_NotAFKViolation: a non-23503 Postgres
// error must not be classified.
func TestClassifyUserCardgroupFKError_NotAFKViolation(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{Code: "23505"} // unique_violation
	if got := classifyUserCardgroupFKError(pgErr); got != nil {
		t.Fatalf("expected nil for non-FK error, got %v", got)
	}
}

// TestClassifyUserCardgroupFKError_NonPgError: a plain stdlib error must not
// be classified.
func TestClassifyUserCardgroupFKError_NonPgError(t *testing.T) {
	t.Parallel()
	if got := classifyUserCardgroupFKError(errors.New("boom")); got != nil {
		t.Fatalf("expected nil for non-pg error, got %v", got)
	}
}

// TestClassifyUserCardgroupFKError_NilError: a nil input must produce nil.
func TestClassifyUserCardgroupFKError_NilError(t *testing.T) {
	t.Parallel()
	if got := classifyUserCardgroupFKError(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
}

// TestClassifyUserCardgroupFKError_LastViewedConstraint: a 23503 violation on
// the users.last_viewed_cardgroup_id constraint must map to ErrCardgroupNotFound
// and also satisfy the legacy ErrNotFound sentinel.
func TestClassifyUserCardgroupFKError_LastViewedConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "users_last_viewed_cardgroup_id_fkey",
	}
	got := classifyUserCardgroupFKError(pgErr)
	if !errors.Is(got, ErrCardgroupNotFound) {
		t.Fatalf("expected ErrCardgroupNotFound, got %v", got)
	}
	if !errors.Is(got, ErrNotFound) {
		t.Fatalf("expected joined ErrNotFound to also match, got %v", got)
	}
}

// TestClassifyUserCardgroupFKError_UnknownConstraint: a 23503 violation on a
// constraint name that does not contain "last_viewed_cardgroup_id" must
// return nil so the caller falls through to eris.Wrap rather than swallowing
// an unrelated FK violation.
func TestClassifyUserCardgroupFKError_UnknownConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "users_some_other_fkey",
	}
	if got := classifyUserCardgroupFKError(pgErr); got != nil {
		t.Fatalf("expected nil for unknown FK constraint, got %v", got)
	}
}
