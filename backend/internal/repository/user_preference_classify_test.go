package repository

// White-box tests for classifyUserPreferenceCardgroupFKError. The function is
// unexported so the tests must live in the same package. All assertions use
// fabricated *pgconn.PgError values — no live DB is required.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// TestClassifyUserPreferenceCardgroupFKError_NotAFKViolation: a non-23503
// Postgres error must not be classified.
func TestClassifyUserPreferenceCardgroupFKError_NotAFKViolation(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{Code: "23505"} // unique_violation
	if got := classifyUserPreferenceCardgroupFKError(pgErr); got != nil {
		t.Fatalf("expected nil for non-FK error, got %v", got)
	}
}

// TestClassifyUserPreferenceCardgroupFKError_NonPgError: a plain stdlib error
// must not be classified.
func TestClassifyUserPreferenceCardgroupFKError_NonPgError(t *testing.T) {
	t.Parallel()
	if got := classifyUserPreferenceCardgroupFKError(errors.New("boom")); got != nil {
		t.Fatalf("expected nil for non-pg error, got %v", got)
	}
}

// TestClassifyUserPreferenceCardgroupFKError_NilError: a nil input must
// produce nil.
func TestClassifyUserPreferenceCardgroupFKError_NilError(t *testing.T) {
	t.Parallel()
	if got := classifyUserPreferenceCardgroupFKError(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
}

// TestClassifyUserPreferenceCardgroupFKError_LastViewedConstraint: a 23503
// violation on a constraint name containing "last_viewed_cardgroup_id" must
// map to ErrCardgroupNotFound and also satisfy the legacy ErrNotFound sentinel.
func TestClassifyUserPreferenceCardgroupFKError_LastViewedConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "user_preferences_last_viewed_cardgroup_id_fkey",
	}
	got := classifyUserPreferenceCardgroupFKError(pgErr)
	if !errors.Is(got, ErrCardgroupNotFound) {
		t.Fatalf("expected ErrCardgroupNotFound, got %v", got)
	}
	if !errors.Is(got, ErrNotFound) {
		t.Fatalf("expected joined ErrNotFound to also match, got %v", got)
	}
}

// TestClassifyUserPreferenceCardgroupFKError_UnknownConstraint: a 23503
// violation on a constraint name that does NOT contain "last_viewed_cardgroup_id"
// must return nil so the caller falls through to eris.Wrap rather than
// swallowing an unrelated FK violation.
func TestClassifyUserPreferenceCardgroupFKError_UnknownConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "user_preferences_user_id_fkey",
	}
	if got := classifyUserPreferenceCardgroupFKError(pgErr); got != nil {
		t.Fatalf("expected nil for unknown FK constraint, got %v", got)
	}
}
