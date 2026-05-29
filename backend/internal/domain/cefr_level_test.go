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
	require.False(t, CEFRUnknown.IsValid())
	require.True(t, CEFRA1.IsValid())
	require.True(t, CEFRC1.IsValid())
}

func TestCEFRLevel_String(t *testing.T) {
	t.Parallel()
	require.Equal(t, "A1", CEFRA1.String())
	require.Equal(t, "B2", CEFRB2.String())
	require.Equal(t, "C1", CEFRC1.String())
	require.Equal(t, "", CEFRUnknown.String())
}

func TestCEFRLevel_Harder(t *testing.T) {
	t.Parallel()
	require.Equal(t, CEFRB2, CEFRA1.Harder(CEFRB2))
	require.Equal(t, CEFRB2, CEFRB2.Harder(CEFRA1))
	require.Equal(t, CEFRA2, CEFRUnknown.Harder(CEFRA2))
	require.Equal(t, CEFRC1, CEFRC1.Harder(CEFRC1))
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
		"A0":   {CEFRUnknown, false},
		"C2":   {CEFRUnknown, false},
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
		"don't":      "don't",
		"o'clock":    "o'clock", // curly apostrophe normalized to straight
		"...":        "",
		"":           "",
	}
	for in, want := range cases {
		require.Equal(t, want, NormalizeWord(in), "input %q", in)
	}
}
