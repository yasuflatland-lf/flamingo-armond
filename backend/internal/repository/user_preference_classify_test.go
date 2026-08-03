package repository

// White-box tests for classifyUserPreferenceCardgroupFKError. Tests live in
// the same package because the function is unexported. All assertions use
// fabricated *pgconn.PgError values — no live DB required.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
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

func TestClassifyUserPreferenceCardgroupFKError_MalformedCardgroupID(t *testing.T) {
	t.Parallel()
	got := classifyUserPreferenceCardgroupFKError(&pgconn.PgError{Code: "22P02"})
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
	discard := slog.New(slog.DiscardHandler)
	got := toDomainUserPreference(context.Background(), discard, gormUserPreference{
		UserID:           "u1",
		LearnDisplayMode: "always_visible",
	})
	if got.LearnDisplayMode != domain.LearnDisplayAlwaysVisible {
		t.Fatalf("got %q, want always_visible", got.LearnDisplayMode)
	}
	gotEmpty := toDomainUserPreference(context.Background(), discard, gormUserPreference{UserID: "u1"})
	if gotEmpty.LearnDisplayMode != domain.DefaultLearnDisplayMode {
		t.Fatalf("empty column: got %q, want default", gotEmpty.LearnDisplayMode)
	}
}

// TestToDomainUserPreference_NewCardRatioAboveCapFallsBackToDefault pins the
// read-path auto-heal: a stored ratio whose new share exceeds the 80% review floor
// (19/20 = 95% new) could pass the previous loose column CHECK but is rejected
// by domain.ParseNewCardRatio, so toDomainUserPreference normalizes it to
// DefaultNewCardRatio on read — existing FSRS-breaking rows self-heal with no DB
// migration. An accepted stored ratio is preserved. No live DB required.
func TestToDomainUserPreference_NewCardRatioAboveCapFallsBackToDefault(t *testing.T) {
	t.Parallel()

	discard := slog.New(slog.DiscardHandler)
	healed := toDomainUserPreference(context.Background(), discard, gormUserPreference{
		UserID:          "u1",
		NewCardRatioNum: 19,
		NewCardRatioDen: 20,
	})
	if healed.NewCardRatio != domain.DefaultNewCardRatio {
		t.Fatalf("stored 19/20 (95%% new): got %d/%d, want default %d/%d",
			healed.NewCardRatio.Numerator(), healed.NewCardRatio.Denominator(),
			domain.DefaultNewCardRatio.Numerator(), domain.DefaultNewCardRatio.Denominator())
	}
	if healed.EffectiveNewCardRatio() != domain.DefaultNewCardRatio {
		t.Fatalf("EffectiveNewCardRatio after auto-heal: got %d/%d, want default",
			healed.EffectiveNewCardRatio().Numerator(), healed.EffectiveNewCardRatio().Denominator())
	}

	kept := toDomainUserPreference(context.Background(), discard, gormUserPreference{
		UserID:          "u1",
		NewCardRatioNum: 3,
		NewCardRatioDen: 10,
	})
	if kept.NewCardRatio.Numerator() != 3 || kept.NewCardRatio.Denominator() != 10 {
		t.Fatalf("stored 3/10 (accepted): got %d/%d, want 3/10 preserved",
			kept.NewCardRatio.Numerator(), kept.NewCardRatio.Denominator())
	}
}

// TestToDomainUserPreference_NewCardRatioNonDivisibleDenominatorFallsBackToDefault
// pins the read-path auto-heal for the divisibility rule: a legacy row storing 3/7
// could pass the previous loose column CHECK but is rejected by
// domain.ParseNewCardRatio, so toDomainUserPreference normalizes it to
// DefaultNewCardRatio on read rather than failing the read.
func TestToDomainUserPreference_NewCardRatioNonDivisibleDenominatorFallsBackToDefault(t *testing.T) {
	t.Parallel()

	discard := slog.New(slog.DiscardHandler)
	healed := toDomainUserPreference(context.Background(), discard, gormUserPreference{
		UserID:          "u1",
		NewCardRatioNum: 3,
		NewCardRatioDen: 7,
	})
	if healed.NewCardRatio != domain.DefaultNewCardRatio {
		t.Fatalf("stored 3/7 (denominator does not divide the default session): got %d/%d, want default %d/%d",
			healed.NewCardRatio.Numerator(), healed.NewCardRatio.Denominator(),
			domain.DefaultNewCardRatio.Numerator(), domain.DefaultNewCardRatio.Denominator())
	}
}

// TestToDomainUserPreference_RejectedRatioLogsWarn pins the read-path WARN: a
// stored ratio the value object rejects still degrades to DefaultNewCardRatio,
// but the offending pair now appears in the log instead of vanishing silently.
func TestToDomainUserPreference_RejectedRatioLogsWarn(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	got := toDomainUserPreference(context.Background(), logger, gormUserPreference{
		UserID:          "11111111-1111-1111-1111-111111111111",
		NewCardRatioNum: 19,
		NewCardRatioDen: 20,
	})
	if got.NewCardRatio != domain.DefaultNewCardRatio {
		t.Fatalf("fallback: got %d/%d, want default",
			got.NewCardRatio.Numerator(), got.NewCardRatio.Denominator())
	}

	line := buf.String()
	if gotLines := strings.Split(strings.TrimSpace(line), "\n"); len(gotLines) != 1 {
		t.Fatalf("expected exactly one WARN line, got %d: %s", len(gotLines), line)
	}
	for _, want := range []string{
		"stored new card ratio rejected",
		`"new_card_ratio_num":19`,
		`"new_card_ratio_den":20`,
		`"user_id":"11111111-1111-1111-1111-111111111111"`,
		"error_chain",
	} {
		if !strings.Contains(line, want) {
			t.Fatalf("WARN line missing %q; got: %s", want, line)
		}
	}
}

// TestToDomainUserPreference_AcceptedRatioLogsNothing is the negative half: a
// ratio the value object accepts must not produce a log line at all.
func TestToDomainUserPreference_AcceptedRatioLogsNothing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	got := toDomainUserPreference(context.Background(), logger, gormUserPreference{
		UserID:          "22222222-2222-2222-2222-222222222222",
		NewCardRatioNum: 3,
		NewCardRatioDen: 10,
	})
	if got.NewCardRatio.Numerator() != 3 || got.NewCardRatio.Denominator() != 10 {
		t.Fatalf("got %d/%d, want 3/10",
			got.NewCardRatio.Numerator(), got.NewCardRatio.Denominator())
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log output for an accepted ratio, got: %s", buf.String())
	}
}

// TestToDomainUserPreference_UnsetRatioLogsNothing protects the legacy zero
// sentinel from generating a WARN on every read of a row without a set ratio.
func TestToDomainUserPreference_UnsetRatioLogsNothing(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	got := toDomainUserPreference(context.Background(), logger, gormUserPreference{
		UserID: "33333333-3333-3333-3333-333333333333",
	})
	if got.NewCardRatio != domain.DefaultNewCardRatio {
		t.Fatalf("fallback: got %d/%d, want default",
			got.NewCardRatio.Numerator(), got.NewCardRatio.Denominator())
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log output for an unset ratio, got: %s", buf.String())
	}
}
