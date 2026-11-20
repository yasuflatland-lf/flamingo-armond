package domain

import (
	"maps"
	"slices"
	"testing"
	"unicode"

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

// normalizeWordCases maps each input to its expected NormalizeWord output.
var normalizeWordCases = map[string]string{
	"Water":      "water",
	"  WATER!  ": "water",
	"(bank)":     "bank",
	"ice cream":  "ice cream",
	"Ice  Cream": "ice cream", // internal whitespace run collapsed
	"don't":      "don't",     // ASCII apostrophe preserved internally
	"o'clock":    "o'clock",   // ASCII apostrophe preserved internally
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
	// Every unicode.IsSpace run (tab, NBSP, U+3000, newline) collapses to one space.
	"artificial\tintelligence":      "artificial intelligence",
	"artificial\u00a0intelligence":  "artificial intelligence",
	"artificial\u3000intelligence":  "artificial intelligence",
	" Artificial \n Intelligence! ": "artificial intelligence",
	"\u0130\u0308":                  "\u00ef", // U+0130 lowers to "i", then composes with U+0308
	// Whitespace mixed with edge punctuation or symbols must be trimmed to the word, not left as " bank ".
	"( bank )":                   "bank",
	"! water":                    "water",
	"water!\u00a0":               "water",
	"~water~":                    "water",
	"$water":                     "water",
	"artificial\r\nintelligence": "artificial intelligence",
	// The first NFC makes canonically equivalent inputs share one key; NFKC is rejected, so ligatures stay.
	"I\u0307":  "i",
	"\u0130":   "i",
	"\ufb01ne": "\ufb01ne",
}

// normalizeWordIdempotenceInputs are known non-idempotence counterexamples of
// the single-NFC form plus canonical-equivalence singletons (Angstrom, Kelvin
// and Ohm signs) and whitespace edge cases.
var normalizeWordIdempotenceInputs = []string{
	"\u0130\u0308", "\u0130\u0301", "\u212b", "\u212a", "\u2126", "a  b", "\t'x'\t",
}

// normalizeWordSeeds returns every table input plus the idempotence inputs.
func normalizeWordSeeds() []string {
	return append(slices.Collect(maps.Keys(normalizeWordCases)), normalizeWordIdempotenceInputs...)
}

func TestNormalizeWord(t *testing.T) {
	t.Parallel()
	for in, want := range normalizeWordCases {
		require.Equal(t, want, NormalizeWord(in), "input %q", in)
	}
}

// Every unicode.IsSpace rune, not a hand-picked few, collapses to one ASCII space.
func TestNormalizeWord_CollapsesEveryUnicodeSpaceRune(t *testing.T) {
	t.Parallel()
	for r := rune(0); r <= 0xFFFF; r++ {
		if !unicode.IsSpace(r) {
			continue
		}
		sep := string(r)
		require.Equal(t, "a b", NormalizeWord("a"+sep+sep+"b"), "separator U+%04X", r)
	}
}

func TestNormalizeWord_Idempotent(t *testing.T) {
	t.Parallel()
	for _, in := range normalizeWordSeeds() {
		once := NormalizeWord(in)
		require.Equal(t, once, NormalizeWord(once), "input %q", in)
	}
}

func FuzzNormalizeWord_Idempotent(f *testing.F) {
	for _, in := range normalizeWordSeeds() {
		f.Add(in)
	}
	f.Fuzz(func(t *testing.T, s string) {
		n := NormalizeWord(s)
		if NormalizeWord(n) != n {
			t.Fatalf("NormalizeWord not idempotent for %q: %q -> %q", s, n, NormalizeWord(n))
		}
	})
}
