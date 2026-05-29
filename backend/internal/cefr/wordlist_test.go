package cefr

import (
	"testing"

	"backend/internal/domain"

	"github.com/stretchr/testify/require"
)

func TestParseMarkdown_HappyPath(t *testing.T) {
	t.Parallel()
	src := "## A1\n- cat\n- ice cream\n\n## B2\n- nevertheless\n"
	out, err := ParseMarkdown(src)
	require.NoError(t, err)
	require.Equal(t, domain.CEFRA1, out["cat"])
	require.Equal(t, domain.CEFRA1, out["ice cream"])
	require.Equal(t, domain.CEFRB2, out["nevertheless"])
	require.Len(t, out, 3)
}

func TestParseMarkdown_DuplicateKeyHighestWins(t *testing.T) {
	t.Parallel()
	src := "## A1\n- run\n\n## B1\n- run\n"
	out, err := ParseMarkdown(src)
	require.NoError(t, err)
	require.Equal(t, domain.CEFRB1, out["run"])
}

func TestParseMarkdown_BulletBeforeHeading(t *testing.T) {
	t.Parallel()
	_, err := ParseMarkdown("- orphan\n")
	require.Error(t, err)
}

func TestParseMarkdown_BadHeading(t *testing.T) {
	t.Parallel()
	_, err := ParseMarkdown("## Z9\n- x\n")
	require.Error(t, err)
}

func TestParseMarkdown_AnnotationStripped(t *testing.T) {
	t.Parallel()
	src := "## B1\n- run _(verb)\n- carry on _(phrasal verb)\n"
	out, err := ParseMarkdown(src)
	require.NoError(t, err)
	require.Equal(t, domain.CEFRB1, out["run"])
	require.Equal(t, domain.CEFRB1, out["carry on"])
	_, hasRaw := out["run _(verb)"]
	require.False(t, hasRaw)
}

func TestNewWordList_EmbeddedData(t *testing.T) {
	t.Parallel()
	wl := NewWordList()

	// Known word.
	lvl, ok := wl.Lookup(domain.NormalizeWord("water"))
	require.True(t, ok)
	require.True(t, lvl.IsValid())

	// Multi-word entry round-trips.
	_, ok = wl.Lookup(domain.NormalizeWord("ice cream"))
	require.True(t, ok)

	// No substring match.
	_, ok = wl.Lookup(domain.NormalizeWord("cathedral"))
	require.False(t, ok)

	// Sanity: a non-trivial number of entries loaded.
	require.Greater(t, wl.Len(), 4000)
}

func TestNewWordList_CambridgeC2Data(t *testing.T) {
	t.Parallel()
	wl := NewWordList()

	// Keys verified in data/cambridge-c2.md that are absent from the Oxford
	// lists; each must resolve to exactly C2.
	c2Keys := []string{"ambiguous", "negligible", "paradigm", "bribery", "abrupt"}
	for _, key := range c2Keys {
		lvl, ok := wl.Lookup(domain.NormalizeWord(key))
		require.True(t, ok, "expected C2 key %q to be present", key)
		require.Equal(t, domain.CEFRC2, lvl, "expected %q to map to C2", key)
	}

	// The list must be larger than the Oxford-only baseline (>4000 Oxford + 792 C2).
	require.Greater(t, wl.Len(), 4792)
}
