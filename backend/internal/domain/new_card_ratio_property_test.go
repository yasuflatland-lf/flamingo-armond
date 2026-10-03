package domain

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newCardRatioOracle restates ParseNewCardRatio's contract independently:
// sentinel precedence den<=0, share range, reduced den > max, den does not
// divide the session, share above 4/5.
func newCardRatioOracle(num, den int) (int, int, error) {
	if den <= 0 {
		return 0, 0, ErrNewCardRatioDenominatorNotPositive
	}
	if num <= 0 || num >= den {
		return 0, 0, ErrNewCardRatioShareOutOfRange
	}
	a, b := num, den
	for b != 0 {
		a, b = b, a%b
	}
	rn, rd := num/a, den/a
	switch {
	case rd > NewCardRatioDenMax:
		return 0, 0, ErrNewCardRatioDenominatorTooLarge
	case DefaultLearnSessionSize%rd != 0:
		return 0, 0, ErrNewCardRatioDenominatorNotRepresentable
	case rn*NewCardRatioMaxNewShareDen > NewCardRatioMaxNewShareNum*rd:
		return 0, 0, ErrNewCardRatioNewShareTooHigh
	}
	return rn, rd, nil
}

func checkRatioAgainstOracle(t require.TestingT, num, den int) {
	got, err := ParseNewCardRatio(num, den)
	wn, wd, werr := newCardRatioOracle(num, den)
	if werr != nil {
		require.True(t, errors.Is(err, werr), "ParseNewCardRatio(%d, %d) = %v, want %v", num, den, err, werr)
		require.True(t, got.IsZero(), "rejected input must return the zero value")
		return
	}
	require.NoError(t, err, "ParseNewCardRatio(%d, %d)", num, den)
	require.Equal(t, [2]int{wn, wd}, [2]int{got.Numerator(), got.Denominator()})
	require.Equal(t, got.Denominator(), got.Numerator()+got.ReviewShare())
	again, err := ParseNewCardRatio(got.Numerator(), got.Denominator())
	require.NoError(t, err)
	require.Equal(t, got, again, "an accepted ratio must be a fixed point")
}

// TestParseNewCardRatio_Property_MatchesOracleExhaustively enumerates every
// (num, den) in [-5, 120]^2, which covers every reduced denominator up to 120.
func TestParseNewCardRatio_Property_MatchesOracleExhaustively(t *testing.T) {
	t.Parallel()
	for num := -5; num <= 120; num++ {
		for den := -5; den <= 120; den++ {
			checkRatioAgainstOracle(t, num, den)
		}
	}
}

// TestParseNewCardRatio_Property_ScaleInvariantAndTotal covers the int range the
// exhaustive grid cannot: extremes never panic, and k*num/k*den parses exactly
// like num/den.
func TestParseNewCardRatio_Property_ScaleInvariantAndTotal(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		edge := rapid.SampledFrom([]int{math.MinInt, math.MinInt + 1, -1, 0, 1, math.MaxInt - 1, math.MaxInt})
		num := rapid.OneOf(rapid.IntRange(-50, 200), edge, rapid.Int()).Draw(t, "num")
		den := rapid.OneOf(rapid.IntRange(-50, 200), edge, rapid.Int()).Draw(t, "den")
		checkRatioAgainstOracle(t, num, den)

		n := rapid.IntRange(-50, 200).Draw(t, "n")
		d := rapid.IntRange(-50, 200).Draw(t, "d")
		k := rapid.IntRange(1, 1000).Draw(t, "k")
		a, ea := ParseNewCardRatio(n, d)
		b, eb := ParseNewCardRatio(k*n, k*d)
		require.Equal(t, a, b)
		require.Equal(t, ea == nil, eb == nil)
		if ea != nil {
			require.ErrorIs(t, eb, ea)
		}
	})
}
