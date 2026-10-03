package domain

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseNewCardRatio_RegressionPins keeps the named inputs behind past
// fixes: 73/100 and 33/100 once reached learn and served zero new cards in a
// 20-card session, and 3/7 was accepted before the divisibility rule; 5/17 is
// the formal model's counterexample.
func TestParseNewCardRatio_RegressionPins(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		num, den int
		wantErr  error
	}{
		{"73/100 served zero new cards in a 20-card session", 73, 100, ErrNewCardRatioDenominatorTooLarge},
		{"33/100 the off-grid example the frontend documented", 33, 100, ErrNewCardRatioDenominatorTooLarge},
		{"5/17 the enumerated counterexample", 5, 17, ErrNewCardRatioDenominatorNotRepresentable},
		{"3/7 was accepted before the divisibility rule", 3, 7, ErrNewCardRatioDenominatorNotRepresentable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseNewCardRatio(tc.num, tc.den)
			require.ErrorIs(t, err, tc.wantErr)
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

func TestDefaultNewCardRatio_IsOneFifth(t *testing.T) {
	t.Parallel()

	require.Equal(t, 1, DefaultNewCardRatio.Numerator())
	require.Equal(t, 5, DefaultNewCardRatio.Denominator())
	require.Equal(t, 4, DefaultNewCardRatio.ReviewShare())
	require.False(t, DefaultNewCardRatio.IsZero())
}

// TestDefaultNewCardRatio_DenominatorDividesSession guards package init:
// DefaultNewCardRatio is built by mustNewCardRatio, which panics on rejection, so a
// divisibility rule that excluded 1/5 would take the whole package down at load.
func TestDefaultNewCardRatio_DenominatorDividesSession(t *testing.T) {
	t.Parallel()

	require.Zero(t, DefaultLearnSessionSize%DefaultNewCardRatio.Denominator())
	_, err := ParseNewCardRatio(DefaultNewCardRatio.Numerator(), DefaultNewCardRatio.Denominator())
	require.NoError(t, err)
}

func TestNewCardRatio_ZeroValueIsZero(t *testing.T) {
	t.Parallel()

	var zero NewCardRatio
	require.True(t, zero.IsZero(), "the zero value must report IsZero")
}
