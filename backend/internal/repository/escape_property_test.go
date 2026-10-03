package repository

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// likeMatch is a reference model of Postgres `s LIKE p` with the default
// ESCAPE '\': % matches any run of characters, _ exactly one, and \x matches x
// literally. It works on runes, as Postgres does for a UTF-8 database.
func likeMatch(s, p string) bool {
	sr, pr := []rune(s), []rune(p)
	memo := map[[2]int]bool{}
	var match func(i, j int) bool
	match = func(i, j int) bool {
		key := [2]int{i, j}
		if v, ok := memo[key]; ok {
			return v
		}
		var res bool
		switch {
		case j == len(pr):
			res = i == len(sr)
		case pr[j] == '%':
			res = match(i, j+1) || (i < len(sr) && match(i+1, j))
		case pr[j] == '_':
			res = i < len(sr) && match(i+1, j+1)
		case pr[j] == '\\' && j+1 < len(pr):
			res = i < len(sr) && sr[i] == pr[j+1] && match(i+1, j+2)
		default:
			res = i < len(sr) && sr[i] == pr[j] && match(i+1, j+1)
		}
		memo[key] = res
		return res
	}
	return match(0, 0)
}

// TestEscapeLikePattern_Property_MatchesLiteralSubstring: for any needle and
// haystack over an alphabet dense in LIKE meta-characters, the pattern
// "%" + escapeLikePattern(needle) + "%" matches exactly when the haystack
// contains the needle as a literal substring. A third of the draws replace
// the needle's % and _ with 'a' in the haystack, so it matches only if a
// meta-character is left unescaped.
func TestEscapeLikePattern_Property_MatchesLiteralSubstring(t *testing.T) {
	t.Parallel()
	wildcardsAsLetters := strings.NewReplacer("_", "a", "%", "a")
	alphabet := rapid.StringOf(rapid.SampledFrom([]rune{'a', 'b', '%', '_', '\\', '\u732b'}))
	rapid.Check(t, func(t *rapid.T) {
		needle := alphabet.Draw(t, "needle")
		var hay string
		switch rapid.IntRange(0, 2).Draw(t, "shape") {
		case 0:
			hay = alphabet.Draw(t, "hay")
		case 1:
			hay = alphabet.Draw(t, "pre") + needle + alphabet.Draw(t, "post")
		default:
			hay = alphabet.Draw(t, "pre") + wildcardsAsLetters.Replace(needle) + alphabet.Draw(t, "post")
		}
		require.Equal(t, strings.Contains(hay, needle), likeMatch(hay, "%"+escapeLikePattern(needle)+"%"),
			"needle %q hay %q", needle, hay)
	})
}
