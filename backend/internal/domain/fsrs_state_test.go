package domain

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

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
