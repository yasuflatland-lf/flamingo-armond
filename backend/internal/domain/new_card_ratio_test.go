package domain

import (
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

	// 99/100 is irreducible with den == NewCardRatioDenMax, so it is accepted.
	r, err := ParseNewCardRatio(99, NewCardRatioDenMax)
	require.NoError(t, err)
	require.Equal(t, 99, r.NewShare())
	require.Equal(t, 1, r.ReviewShare())
}

func TestDefaultNewCardRatio_IsFourFifths(t *testing.T) {
	t.Parallel()

	require.Equal(t, 4, DefaultNewCardRatio.Numerator())
	require.Equal(t, 5, DefaultNewCardRatio.Denominator())
	require.Equal(t, 4, DefaultNewCardRatio.NewShare())
	require.Equal(t, 1, DefaultNewCardRatio.ReviewShare())
	require.False(t, DefaultNewCardRatio.IsZero())
}

func TestNewCardRatio_SharesForThreeSevenths(t *testing.T) {
	t.Parallel()

	// 3/7 is already irreducible: new share 3, review share 4.
	r, err := ParseNewCardRatio(3, 7)
	require.NoError(t, err)
	require.Equal(t, 3, r.Numerator())
	require.Equal(t, 7, r.Denominator())
	require.Equal(t, 3, r.NewShare())
	require.Equal(t, 4, r.ReviewShare())
}

func TestNewCardRatio_ZeroValueIsZero(t *testing.T) {
	t.Parallel()

	var zero NewCardRatio
	require.True(t, zero.IsZero(), "the zero value must report IsZero")
}
