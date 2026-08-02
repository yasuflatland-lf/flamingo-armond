package domain

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseNewCardRatio_ReducesToLowestTerms(t *testing.T) {
	t.Parallel()

	// 8/10 reduces to 4/5 (gcd 2). New share 4, review share 1.
	r, err := ParseNewCardRatio(8, 10)
	require.NoError(t, err)
	require.Equal(t, 4, r.Numerator())
	require.Equal(t, 5, r.Denominator())
	require.Equal(t, 4, r.NewShare())
	require.Equal(t, 1, r.ReviewShare())
}

func TestParseNewCardRatio_RejectsOutOfBounds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		num, den int
		wantErr  error
	}{
		{"zero denominator", 1, 0, ErrNewCardRatioDenominatorNotPositive},
		{"negative denominator", 1, -5, ErrNewCardRatioDenominatorNotPositive},
		{"zero numerator", 0, 5, ErrNewCardRatioShareOutOfRange},
		{"negative numerator", -1, 5, ErrNewCardRatioShareOutOfRange},
		{"numerator equals denominator", 3, 3, ErrNewCardRatioShareOutOfRange},
		{"numerator exceeds denominator", 5, 3, ErrNewCardRatioShareOutOfRange},
		{"reduced denominator over max", 50, 101, ErrNewCardRatioDenominatorTooLarge},
		{"denominator over max only after reduction", 3, 303, ErrNewCardRatioDenominatorTooLarge},
		{"73/100 served zero new cards in a 20-card session", 73, 100, ErrNewCardRatioDenominatorTooLarge},
		{"79/100 the worst den=100 case", 79, 100, ErrNewCardRatioDenominatorTooLarge},
		{"33/100 the off-grid example the frontend documented", 33, 100, ErrNewCardRatioDenominatorTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseNewCardRatio(tc.num, tc.den)
			require.Error(t, err)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestParseNewCardRatio_AllowsMaxDenominator(t *testing.T) {
	t.Parallel()

	// 13/20 is irreducible with den == NewCardRatioDenMax and a new share (65%)
	// below the 80% ceiling, so it is accepted at the boundary.
	r, err := ParseNewCardRatio(13, NewCardRatioDenMax)
	require.NoError(t, err)
	require.Equal(t, 13, r.NewShare())
	require.Equal(t, 7, r.ReviewShare())
}

func TestParseNewCardRatio_RejectsNewShareAboveCap(t *testing.T) {
	t.Parallel()

	// Every case reduces to a denominator that divides the default session, so
	// the share cap — not the divisibility check — is the rule that fires.
	cases := []struct {
		name     string
		num, den int
	}{
		{"95% reduces to 19/20", 95, 100},
		{"90% reduces to 9/10", 90, 100},
		{"85% reduces to 17/20", 85, 100},
		{"9/10 irreducible", 9, 10},
		{"17/20 irreducible", 17, 20},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseNewCardRatio(tc.num, tc.den)
			require.ErrorIs(t, err, ErrNewCardRatioNewShareTooHigh)
			require.NotErrorIs(t, err, ErrNewCardRatioDenominatorNotRepresentable)
		})
	}
}

// TestParseNewCardRatio_RejectsDenominatorNotDividingSession pins the divisibility
// rule: a reduced denominator inside the cap but not a divisor of the default
// session is rejected, because a session of that size cannot split into the stored
// ratio exactly.
func TestParseNewCardRatio_RejectsDenominatorNotDividingSession(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		num, den int
	}{
		{"5/17 the enumerated counterexample", 5, 17},
		{"1/3", 1, 3},
		{"2/7", 2, 7},
		{"3/7 was accepted before the divisibility rule", 3, 7},
		{"6/9 reduces to 2/3, so reduction runs first", 6, 9},
		{"1/19 the largest non-dividing denominator under the cap", 1, 19},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseNewCardRatio(tc.num, tc.den)
			require.ErrorIs(t, err, ErrNewCardRatioDenominatorNotRepresentable)
			require.NotErrorIs(t, err, ErrNewCardRatioDenominatorTooLarge)
		})
	}
}

// TestParseNewCardRatio_DenominatorCheckPrecedence pins the check order: an
// over-cap denominator reports the coarser "too large" reason even though it also
// fails divisibility, and a non-dividing denominator reports the structural reason
// even when its share also exceeds the 4/5 cap.
func TestParseNewCardRatio_DenominatorCheckPrecedence(t *testing.T) {
	t.Parallel()

	overCap := []struct {
		name     string
		num, den int
	}{
		{"1/21 one past the cap", 1, 21},
		{"1/40 well past the cap", 1, 40},
		{"3/303 reduces to 1/101", 3, 303},
	}
	for _, tc := range overCap {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseNewCardRatio(tc.num, tc.den)
			require.ErrorIs(t, err, ErrNewCardRatioDenominatorTooLarge)
			require.NotErrorIs(t, err, ErrNewCardRatioDenominatorNotRepresentable)
		})
	}

	shareAlsoTooHigh := []struct {
		name     string
		num, den int
	}{
		{"7/8 is 87.5% new and 8 does not divide 20", 7, 8},
		{"5/6 is 83.3% new and 6 does not divide 20", 5, 6},
	}
	for _, tc := range shareAlsoTooHigh {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseNewCardRatio(tc.num, tc.den)
			require.ErrorIs(t, err, ErrNewCardRatioDenominatorNotRepresentable)
			require.NotErrorIs(t, err, ErrNewCardRatioNewShareTooHigh)
		})
	}
}

// TestParseNewCardRatio_AcceptsEveryDividingDenominator enumerates the complete
// accepted set: every numerator coprime to a divisor of the default session whose
// share stays at or below 4/5. Each pair is already reduced, so ParseNewCardRatio
// must round-trip it unchanged.
func TestParseNewCardRatio_AcceptsEveryDividingDenominator(t *testing.T) {
	t.Parallel()

	accepted := []struct{ num, den int }{
		{1, 2},
		{1, 4}, {3, 4},
		{1, 5}, {2, 5}, {3, 5}, {4, 5},
		{1, 10}, {3, 10}, {7, 10},
		{1, 20}, {3, 20}, {7, 20}, {9, 20}, {11, 20}, {13, 20},
	}
	for _, tc := range accepted {
		t.Run(fmt.Sprintf("%d/%d", tc.num, tc.den), func(t *testing.T) {
			t.Parallel()
			r, err := ParseNewCardRatio(tc.num, tc.den)
			require.NoError(t, err)
			require.Equal(t, tc.num, r.Numerator())
			require.Equal(t, tc.den, r.Denominator())
			require.Equal(t, tc.num, r.NewShare())
			require.Equal(t, tc.den-tc.num, r.ReviewShare())
			require.Zero(t, DefaultLearnSessionSize%r.Denominator(),
				"an accepted denominator must divide the default session")
		})
	}
}

// TestParseNewCardRatio_AcceptsEveryFrontendSelectableShare pins backend/UI parity:
// the profile control emits percent/100 in 5-point steps over [5, 80], and every
// one of those 16 values must still parse. A future UI step change breaks this test
// rather than production.
func TestParseNewCardRatio_AcceptsEveryFrontendSelectableShare(t *testing.T) {
	t.Parallel()

	cases := []struct {
		percent          int
		wantNum, wantDen int
	}{
		{5, 1, 20},
		{10, 1, 10},
		{15, 3, 20},
		{20, 1, 5},
		{25, 1, 4},
		{30, 3, 10},
		{35, 7, 20},
		{40, 2, 5},
		{45, 9, 20},
		{50, 1, 2},
		{55, 11, 20},
		{60, 3, 5},
		{65, 13, 20},
		{70, 7, 10},
		{75, 3, 4},
		{80, 4, 5},
	}
	require.Len(t, cases, 16, "the 5%-step control over [5, 80] has exactly 16 positions")
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d%%", tc.percent), func(t *testing.T) {
			t.Parallel()
			r, err := ParseNewCardRatio(tc.percent, 100)
			require.NoError(t, err)
			require.Equal(t, tc.wantNum, r.Numerator())
			require.Equal(t, tc.wantDen, r.Denominator())
		})
	}
}

func TestParseNewCardRatio_AllowsBoundaryAndBelowNewShare(t *testing.T) {
	t.Parallel()

	// 4/5 (80%) is the inclusive boundary; 3/4 (75%) and 1/20 (5%) sit below it.
	cases := []struct{ num, den int }{
		{4, 5},
		{3, 4},
		{1, 20},
	}
	for _, tc := range cases {
		_, err := ParseNewCardRatio(tc.num, tc.den)
		require.NoError(t, err, "ParseNewCardRatio(%d, %d)", tc.num, tc.den)
	}
}

func TestDefaultNewCardRatio_IsFourFifths(t *testing.T) {
	t.Parallel()

	require.Equal(t, 4, DefaultNewCardRatio.Numerator())
	require.Equal(t, 5, DefaultNewCardRatio.Denominator())
	require.Equal(t, 4, DefaultNewCardRatio.NewShare())
	require.Equal(t, 1, DefaultNewCardRatio.ReviewShare())
	require.False(t, DefaultNewCardRatio.IsZero())
}

// TestDefaultNewCardRatio_DenominatorDividesSession guards package init:
// DefaultNewCardRatio is built by mustNewCardRatio, which panics on rejection, so a
// divisibility rule that excluded 4/5 would take the whole package down at load.
func TestDefaultNewCardRatio_DenominatorDividesSession(t *testing.T) {
	t.Parallel()

	require.Zero(t, DefaultLearnSessionSize%DefaultNewCardRatio.Denominator())
	_, err := ParseNewCardRatio(DefaultNewCardRatio.Numerator(), DefaultNewCardRatio.Denominator())
	require.NoError(t, err)
}

func TestNewCardRatio_SharesForThreeTenths(t *testing.T) {
	t.Parallel()

	// 3/10 is already irreducible: new share 3, review share 7.
	r, err := ParseNewCardRatio(3, 10)
	require.NoError(t, err)
	require.Equal(t, 3, r.Numerator())
	require.Equal(t, 10, r.Denominator())
	require.Equal(t, 3, r.NewShare())
	require.Equal(t, 7, r.ReviewShare())
}

func TestNewCardRatio_ZeroValueIsZero(t *testing.T) {
	t.Parallel()

	var zero NewCardRatio
	require.True(t, zero.IsZero(), "the zero value must report IsZero")
}
