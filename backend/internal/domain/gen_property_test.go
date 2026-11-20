package domain

import (
	"strings"
	"time"

	"pgregory.net/rapid"
)

// textAtoms mixes ASCII, whitespace strings.TrimSpace removes (U+3000, NBSP,
// U+0085) and does not (ZWSP, U+FEFF), CJK, and multi-rune grapheme clusters,
// so byte, rune and grapheme counts all differ.
var textAtoms = []string{
	" ", "\t", "\n", "\r\n", "\u3000", "\u00a0", "\u200b", "\u0301",
	"a", "Z", "!", "_", "%", "\\", "\u732b", "e\u0301", "\U0001F44D",
	"\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466", "\ufeff", "\u0085",
}

// genText returns up to maxAtoms atoms. One draw in four prepends
// capHint-1, capHint or capHint+1 copies of a one-grapheme unit (ASCII or a ZWJ
// family emoji) so both sides of the grapheme cap are reached every run.
func genText(maxAtoms, capHint int) *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		atoms := rapid.SliceOfN(rapid.SampledFrom(textAtoms), 0, maxAtoms).Draw(t, "atoms")
		s := strings.Join(atoms, "")
		if capHint > 0 && rapid.IntRange(0, 3).Draw(t, "pad") == 0 {
			n := rapid.IntRange(capHint-1, capHint+1).Draw(t, "n")
			unit := rapid.SampledFrom([]string{"a", "\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466"}).Draw(t, "unit")
			s = strings.Repeat(unit, n) + s
		}
		return s
	})
}

// genInstant spans 1970..2100 at nanosecond resolution in one of four fixed
// zones, so JST, UTC and far-offset operands all appear.
func genInstant() *rapid.Generator[time.Time] {
	zones := []*time.Location{time.UTC, learnDayZone, time.FixedZone("m11", -11*3600), time.FixedZone("p14", 14*3600)}
	return rapid.Custom(func(t *rapid.T) time.Time {
		sec := rapid.Int64Range(0, 4102444800).Draw(t, "sec")
		ns := rapid.Int64Range(0, 999_999_999).Draw(t, "ns")
		return time.Unix(sec, ns).In(rapid.SampledFrom(zones).Draw(t, "zone"))
	})
}
