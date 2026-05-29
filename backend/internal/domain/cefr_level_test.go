package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCEFRLevel_RankAndValid(t *testing.T) {
	t.Parallel()
	require.Equal(t, 0, CEFRUnknown.Rank())
	require.Equal(t, 1, CEFRA1.Rank())
	require.Equal(t, 5, CEFRC1.Rank())
	require.Equal(t, 6, CEFRC2.Rank())
	require.True(t, CEFRC2.Rank() > CEFRC1.Rank())
	require.False(t, CEFRUnknown.IsValid())
	require.True(t, CEFRA1.IsValid())
	require.True(t, CEFRC1.IsValid())
	require.True(t, CEFRC2.IsValid())
}

func TestCEFRLevel_String(t *testing.T) {
	t.Parallel()
	require.Equal(t, "A1", CEFRA1.String())
	require.Equal(t, "B2", CEFRB2.String())
	require.Equal(t, "C1", CEFRC1.String())
	require.Equal(t, "C2", CEFRC2.String())
	require.Equal(t, "", CEFRUnknown.String())
}

func TestCEFRLevel_Harder(t *testing.T) {
	t.Parallel()
	require.Equal(t, CEFRB2, CEFRA1.Harder(CEFRB2))
	require.Equal(t, CEFRB2, CEFRB2.Harder(CEFRA1))
	require.Equal(t, CEFRA2, CEFRUnknown.Harder(CEFRA2))
	require.Equal(t, CEFRC1, CEFRC1.Harder(CEFRC1))
	require.Equal(t, CEFRC2, CEFRC1.Harder(CEFRC2))
	require.Equal(t, CEFRC2, CEFRC2.Harder(CEFRC1))
}

func TestParseCEFRLevel(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		want CEFRLevel
		ok   bool
	}{
		"A1":   {CEFRA1, true},
		"c1":   {CEFRC1, true},
		" b1 ": {CEFRB1, true},
		"C2":   {CEFRC2, true},
		"c2":   {CEFRC2, true},
		" C2 ": {CEFRC2, true},
		"A0":   {CEFRUnknown, false},
		"C3":   {CEFRUnknown, false},
		"D1":   {CEFRUnknown, false},
		"":     {CEFRUnknown, false},
	}
	for in, exp := range cases {
		got, ok := ParseCEFRLevel(in)
		require.Equal(t, exp.ok, ok, "input %q", in)
		require.Equal(t, exp.want, got, "input %q", in)
	}
}

func TestNormalizeWord(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Water":      "water",
		"  WATER!  ": "water",
		"(bank)":     "bank",
		"ice cream":  "ice cream",
		"Ice  Cream": "ice  cream", // internal whitespace preserved verbatim
		"don't":      "don't",      // ASCII apostrophe preserved internally
		"o'clock":    "o'clock",    // ASCII apostrophe preserved internally
		// Explicit unicode: U+2019 right curly apostrophe folded to straight.
		// After folding the U+2019 → U+0027, TrimFunc strips a peripheral
		// straight apostrophe; internal ones are kept.
		"’clock":    "clock",     // U+2019 at start stripped after fold
		"o’clock":   "o'clock",   // U+2019 inside word folded to straight
		"don’t":     "don't",     // U+2019 inside word folded to straight
		"don‘t":     "don't",     // U+2018 left single quote folded to straight
		"carry’’on": "carry''on", // two consecutive curly apostrophes folded
		"...":       "",
		"":          "",
	}
	for in, want := range cases {
		require.Equal(t, want, NormalizeWord(in), "input %q", in)
	}
}
