package repository

// White-box tests for classifyFKError. The function is unexported so the
// tests must live in the same package. All assertions use fabricated
// *pgconn.PgError values — no live DB is required.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyFKError_NotAFKViolation(t *testing.T) {
	t.Parallel()
	// A non-23503 Postgres error must not be classified.
	pgErr := &pgconn.PgError{Code: "23505"} // unique_violation
	if got := classifyFKError(pgErr); got != nil {
		t.Fatalf("expected nil for non-FK error, got %v", got)
	}
}

func TestClassifyFKError_NonPgError(t *testing.T) {
	t.Parallel()
	// A plain stdlib error must not be classified.
	if got := classifyFKError(errors.New("some error")); got != nil {
		t.Fatalf("expected nil for non-pg error, got %v", got)
	}
}

func TestClassifyFKError_UserIDConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "user_roles_user_id_fkey",
	}
	got := classifyFKError(pgErr)
	if !errors.Is(got, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound for user_id FK, got %v", got)
	}
	// Must also satisfy the legacy sentinel.
	if !errors.Is(got, ErrNotFound) {
		t.Fatalf("expected errors.Is(_, ErrNotFound) true for user_id FK, got %v", got)
	}
}

func TestClassifyFKError_RoleIDConstraint(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "user_roles_role_id_fkey",
	}
	got := classifyFKError(pgErr)
	if !errors.Is(got, ErrRoleNotFound) {
		t.Fatalf("expected ErrRoleNotFound for role_id FK, got %v", got)
	}
	if !errors.Is(got, ErrNotFound) {
		t.Fatalf("expected errors.Is(_, ErrNotFound) true for role_id FK, got %v", got)
	}
}

func TestClassifyFKError_UnknownConstraint(t *testing.T) {
	t.Parallel()
	// A FK violation on a constraint name that does not match user_id or role_id
	// must return nil so the caller falls through to eris.Wrap.
	pgErr := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "user_roles_other_fkey",
	}
	if got := classifyFKError(pgErr); got != nil {
		t.Fatalf("expected nil for unknown FK constraint, got %v", got)
	}
}

func TestClassifyFKError_NilError(t *testing.T) {
	t.Parallel()
	if got := classifyFKError(nil); got != nil {
		t.Fatalf("expected nil for nil error, got %v", got)
	}
}
