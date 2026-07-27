package domain

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFSRSPhaseIsValid pins the recognised-phase predicate. Every consumer of a
// persisted phase gates on it: the repository read guard rejects the row, and
// FSRSScheduler.Apply panics rather than feeding an unrecognised value to a
// library dispatch that has no default arm. Widening the range would let a
// corrupt column reach both.
func TestFSRSPhaseIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input FSRSPhase
		want  bool
	}{
		{"new", FSRSPhaseNew, true},
		{"learning", FSRSPhaseLearning, true},
		{"review", FSRSPhaseReview, true},
		{"relearning", FSRSPhaseRelearning, true},
		{"below the lowest constant", -1, false},
		{"one past the highest constant", 4, false},
		{"far out of range", 99, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.input.IsValid())
		})
	}
}

// TestIsValidStability pins both sides of the stability predicate. The accept
// side matters as much as the reject side: the repository read guard hard-fails
// a whole query on a false negative, so a narrowing of the predicate must break
// a test here rather than a persisted user's read.
func TestIsValidStability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input float64
		want  bool
	}{
		{"new card placeholder", NewCardStability, true},
		{"smallest positive", math.SmallestNonzeroFloat64, true},
		{"large finite", math.MaxFloat64, true},
		{"zero", 0, false},
		{"negative", -1.5, false},
		{"NaN", math.NaN(), false},
		{"positive infinity", math.Inf(1), false},
		{"negative infinity", math.Inf(-1), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, IsValidStability(tc.input))
		})
	}
}

// TestIsValidDifficulty pins the closed [MinDifficulty, MaxDifficulty] range,
// including both inclusive endpoints. The endpoints are load-bearing: go-fsrs
// clamps difficulty with math.Min(math.Max(d, 1), 10), so exactly 1.0 and
// exactly 10.0 are values the scheduler legitimately emits and persists.
// Narrowing the bounds to an open range would reject those rows.
func TestIsValidDifficulty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input float64
		want  bool
	}{
		{"minimum boundary", MinDifficulty, true},
		{"maximum boundary", MaxDifficulty, true},
		{"new card placeholder", NewCardDifficulty, true},
		{"just below minimum", 0.5, false},
		{"just above maximum", 10.5, false},
		{"zero", 0, false},
		{"negative", -1, false},
		{"NaN", math.NaN(), false},
		{"positive infinity", math.Inf(1), false},
		{"negative infinity", math.Inf(-1), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, IsValidDifficulty(tc.input))
		})
	}
}

// TestRescueMinElapsedMatchesElapsedDayRollover pins the scheduling-credit
// boundary to the rescue window's admission floor. A card admitted at exactly
// rescueMinElapsed must receive one elapsed day of scheduling credit, while a
// card just below that floor must receive none.
//
// The backward-skew row is a full day early on purpose: with a sub-24h skew the
// unguarded expression truncates to the same 0 the guard returns, so deleting
// the guard would leave the row green.
func TestRescueMinElapsedMatchesElapsedDayRollover(t *testing.T) {
	t.Parallel()

	lastReview := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		phase      FSRSPhase
		lastReview time.Time
		now        time.Time
		want       int
	}{
		{"23h59m since last review", FSRSPhaseReview, lastReview, lastReview.Add(23*time.Hour + 59*time.Minute), 0},
		{"exactly 24h since last review", FSRSPhaseReview, lastReview, lastReview.Add(24 * time.Hour), 1},
		{"47h59m since last review", FSRSPhaseReview, lastReview, lastReview.Add(47*time.Hour + 59*time.Minute), 1},
		{"exactly 48h since last review", FSRSPhaseReview, lastReview, lastReview.Add(48 * time.Hour), 2},
		{"new card", FSRSPhaseNew, lastReview, lastReview.Add(48 * time.Hour), 0},
		{"no last review", FSRSPhaseReview, time.Time{}, lastReview, 0},
		{"now a second before last review", FSRSPhaseReview, lastReview, lastReview.Add(-time.Second), 0},
		{"now a full day before last review", FSRSPhaseReview, lastReview, lastReview.Add(-25 * time.Hour), 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := FSRSState{
				Phase:      tc.phase,
				LastReview: tc.lastReview,
			}
			require.Equal(t, tc.want, state.ElapsedDaysAt(tc.now))
		})
	}
}
