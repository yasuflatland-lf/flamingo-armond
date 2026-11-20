package textdic_test

import (
	"cmp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"backend/internal/textdic"
)

// lineAtoms are single-line fragments: ASCII headwords, the whitespace the
// lexer skips (including U+3000), Japanese definition openers, brackets, an
// unrecognized character and invalid UTF-8.
var lineAtoms = []string{
	"cat", "Dog", "a(b", " ", "\t", "\u3000", "\u732b", "\u3044\u306c", "\u30ab", "(x)", "[v]",
	"(", "[", ")", "-", ";", "1", "\u00e9", "!", "@", "\U0001F600", "\uff71", "\xff",
}

func genLine() *rapid.Generator[string] {
	return rapid.Map(rapid.SliceOfN(rapid.SampledFrom(lineAtoms), 0, 6), func(a []string) string { return strings.Join(a, "") })
}

// TestProcess_Property_LinesInRange: for a document of n lines joined by any mix
// of "\n" and "\r\n" (optionally terminated), Process never returns a fatal
// error, every word and every diagnostic points at a line in [1, n], every
// word has a non-empty trimmed front and back, and no diagnostic has the
// Unknown kind.
func TestProcess_Property_LinesInRange(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		lines := rapid.SliceOfN(genLine(), 1, 8).Draw(t, "lines")
		var b strings.Builder
		for i, l := range lines {
			b.WriteString(l)
			if i < len(lines)-1 || rapid.Bool().Draw(t, "terminated") {
				b.WriteString(rapid.SampledFrom([]string{"\n", "\r\n"}).Draw(t, "eol"))
			}
		}
		doc := b.String()
		words, errs, err := textdic.Process(doc)
		require.NoError(t, err)
		for _, w := range words {
			require.True(t, w.Line >= 1 && w.Line <= len(lines), "word %+v outside [1,%d] in %q", w, len(lines), doc)
			require.NotEmpty(t, w.Front)
			require.NotEmpty(t, w.Back)
			require.Equal(t, strings.TrimSpace(w.Front), w.Front)
			require.Equal(t, strings.TrimSpace(w.Back), w.Back)
		}
		for _, e := range errs {
			require.NotEqual(t, textdic.SkipKindUnknown, e.Kind)
			if len(doc) > 0 {
				require.True(t, e.Line >= 1 && e.Line <= len(lines), "error %+v outside [1,%d] in %q", e, len(lines), doc)
			}
		}
	})
}

// skips drops HARD diagnostics and orders the rest by line. HARD errors are
// not line-local: a whitespace-only payload with no terminator yields a
// grammar-level syntax error that the same line inside a larger document does
// not. Process appends lexer and grammar diagnostics to separate lists, so
// only the per-line order is a contract.
func skips(es []textdic.ValidationError) []textdic.ValidationError {
	out := []textdic.ValidationError{}
	for _, e := range es {
		if e.Kind != textdic.SkipKindHard {
			out = append(out, e)
		}
	}
	slices.SortStableFunc(out, func(a, b textdic.ValidationError) int { return cmp.Compare(a.Line, b.Line) })
	return out
}

// TestProcess_Property_LineLocality: parsing a document equals parsing each
// line on its own with Line shifted to the line's position, for words and for
// skip diagnostics, whichever terminator separates the lines.
func TestProcess_Property_LineLocality(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		lines := rapid.SliceOfN(genLine(), 1, 6).Draw(t, "lines")
		eol := rapid.SampledFrom([]string{"\n", "\r\n"}).Draw(t, "eol")
		doc := strings.Join(lines, eol)
		if doc == "" {
			return // the empty payload has its own pinned diagnostic
		}
		words, errs, err := textdic.Process(doc)
		require.NoError(t, err)

		wantWords := []textdic.ParsedWord{}
		wantErrs := []textdic.ValidationError{}
		for i, l := range lines {
			if l == "" {
				continue
			}
			ws, es, err := textdic.Process(l)
			require.NoError(t, err)
			for _, w := range ws {
				w.Line = i + 1
				wantWords = append(wantWords, w)
			}
			for _, e := range skips(es) {
				e.Line = i + 1
				wantErrs = append(wantErrs, e)
			}
		}
		require.Equal(t, wantWords, words)
		require.Equal(t, wantErrs, skips(errs))
	})
}

// FuzzProcess: arbitrary bytes never produce a fatal error or an Unknown-kind
// diagnostic, and every word carries both sides.
func FuzzProcess(f *testing.F) {
	for _, s := range []string{"", "apple \u308a\u3093\u3054", "a \u732b\r\n\r\nb \u72ac", "@broken\n", "orphan", "\r\n[", "\xff\xfe"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		words, errs, err := textdic.Process(s)
		require.NoError(t, err)
		for _, w := range words {
			require.NotEmpty(t, w.Front)
			require.NotEmpty(t, w.Back)
		}
		for _, e := range errs {
			require.NotEqual(t, textdic.SkipKindUnknown, e.Kind)
		}
	})
}
