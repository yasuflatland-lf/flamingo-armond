package repository

// White-box tests for classifyUserPreferenceCardgroupFKError. Tests live in
// the same package because the function is unexported. All assertions use
// fabricated *pgconn.PgError values — no live DB required.

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"backend/internal/domain"
)

func TestClassifyUserPreferenceCardgroupFKError_NotAFKViolation(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{Code: "23505"} // unique_violation
	if got := classifyUserPreferenceCardgroupFKError(pgErr); got != nil {
		t.Fatalf("expected nil for non-FK error, got %v", got)
	}
}

func TestClassifyUserPreferenceCardgroupFKError_NonPgError(t *testing.T) {
	t.Parallel()
	if got := classifyUserPreferenceCardgroupFKError(errors.New("boom")); got != nil {
		t.Fatalf("expected nil for non-pg error, got %v", got)
	}
}

func TestClassifyUserPreferenceCardgroupFKError_NilError(t *testing.T) {
	t.Parallel()
	if got := classifyUserPreferenceCardgroupFKError(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
}

// TestClassifyUserPreferenceCardgroupFKError_LastViewedConstraint verifies that
// a 23503 violation on the last_viewed_cardgroup_id FK maps to
// ErrCardgroupNotFound and also satisfies the joined ErrNotFound sentinel.
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

// TestClassifyUserPreferenceCardgroupFKError_UnknownConstraint verifies that a
// 23503 on an unrelated constraint returns nil so the caller falls through to
// eris.Wrap rather than swallowing the violation.
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

// TestToDomainUserPreference_LearnDisplayMode verifies that toDomainUserPreference
// maps a stored learn_display_mode string to the correct domain constant, and that
// an empty or unknown column value falls back to DefaultLearnDisplayMode.
// No live DB required — only the mapping function is exercised.
func TestToDomainUserPreference_LearnDisplayMode(t *testing.T) {
	t.Parallel()
	got := toDomainUserPreference(gormUserPreference{
		UserID:           "u1",
		LearnDisplayMode: "always_visible",
	})
	if got.LearnDisplayMode != domain.LearnDisplayAlwaysVisible {
		t.Fatalf("got %q, want always_visible", got.LearnDisplayMode)
	}
	gotEmpty := toDomainUserPreference(gormUserPreference{UserID: "u1"})
	if gotEmpty.LearnDisplayMode != domain.DefaultLearnDisplayMode {
		t.Fatalf("empty column: got %q, want default", gotEmpty.LearnDisplayMode)
	}
}
