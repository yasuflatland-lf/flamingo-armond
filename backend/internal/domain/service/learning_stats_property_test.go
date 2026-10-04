package service

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"backend/internal/domain"
)

func genFSRSStats() *rapid.Generator[[]domain.FSRSStat] {
	return rapid.Custom(func(t *rapid.T) []domain.FSRSStat {
		n := rapid.IntRange(0, 30).Draw(t, "n")
		out := make([]domain.FSRSStat, n)
		for i := range out {
			out[i] = domain.FSRSStat{
				CardID:      fmt.Sprintf("c%02d", i),
				CardgroupID: rapid.SampledFrom([]string{"cg0", "cg1", "cg2", "cg3"}).Draw(t, "cg"),
				Phase:       domain.FSRSPhase(rapid.IntRange(0, 3).Draw(t, "phase")),
				Stability: rapid.OneOf(
					rapid.Float64Range(0, 40),
					rapid.SampledFrom([]float64{domain.LearnedStabilityDays, domain.MatureStabilityDays}),
				).Draw(t, "stability"),
				Lapses: rapid.IntRange(0, 4).Draw(t, "lapses"),
			}
		}
		return out
	})
}

// TestAggregateMastery_Property_PartitionsStats: every stat lands in exactly the
// tier ClassifyMastery gives it; one DeckMastery per totals key, sorted by id,
// whose learned/mature counts are the per-deck tier counts.
func TestAggregateMastery_Property_PartitionsStats(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		stats := genFSRSStats().Draw(t, "stats")
		totals := rapid.MapOf(rapid.SampledFrom([]string{"cg0", "cg1", "cg2", "cg9"}), rapid.IntRange(1, 50)).Draw(t, "totals")

		var want MasteryBreakdown
		want.TotalStudied = len(stats)
		perDeck := map[string][2]int{}
		for _, s := range stats {
			acc := perDeck[s.CardgroupID]
			switch domain.ClassifyMastery(domain.FSRSState{Stability: s.Stability}, domain.LearnedStabilityDays, domain.MatureStabilityDays) {
			case domain.TierInProgress:
				want.InProgress++
			case domain.TierLearned:
				want.Learned++
				acc[0]++
			case domain.TierMature:
				want.Mature++
				acc[1]++
			}
			perDeck[s.CardgroupID] = acc
		}
		ids := make([]string, 0, len(totals))
		for id := range totals {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		wantDecks := make([]DeckMastery, 0, len(ids))
		for _, id := range ids {
			wantDecks = append(wantDecks, DeckMastery{CardgroupID: id, TotalCards: totals[id], LearnedCards: perDeck[id][0], MatureCards: perDeck[id][1]})
		}

		got, decks, err := AggregateMastery(stats, totals)
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.Equal(t, wantDecks, decks)
	})
}

// TestTopStruggling_Property_FilterSortCap: the result is non-nil, holds only
// lapsed stats copied verbatim, is ordered by (Lapses desc, Stability asc) with
// ties in input order, has min(limit, #lapsed) rows, and no dropped lapsed stat
// ranks strictly ahead of the last kept row.
func TestTopStruggling_Property_FilterSortCap(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		stats := genFSRSStats().Draw(t, "stats")
		limit := rapid.IntRange(0, 12).Draw(t, "limit")
		ahead := func(a, b StrugglingCard) bool { // a strictly outranks b
			return a.Lapses > b.Lapses || (a.Lapses == b.Lapses && a.Stability < b.Stability)
		}
		index := map[string]int{}
		lapsed := 0
		for i, s := range stats {
			index[s.CardID] = i
			if s.Lapses >= 1 {
				lapsed++
			}
		}

		got := TopStruggling(stats, limit)
		require.NotNil(t, got)
		require.Len(t, got, min(limit, lapsed))
		kept := map[string]bool{}
		for i, c := range got {
			src := stats[index[c.CardID]]
			require.GreaterOrEqual(t, src.Lapses, 1)
			require.Equal(t, StrugglingCard{CardID: src.CardID, Lapses: src.Lapses, Stability: src.Stability}, c)
			if i > 0 {
				prev := got[i-1]
				require.False(t, ahead(c, prev), "row %d outranks row %d", i, i-1)
				if !ahead(prev, c) {
					require.Less(t, index[prev.CardID], index[c.CardID], "ties keep input order")
				}
			}
			kept[c.CardID] = true
		}
		if len(got) > 0 {
			last := got[len(got)-1]
			for _, s := range stats {
				if s.Lapses >= 1 && !kept[s.CardID] {
					require.False(t, ahead(StrugglingCard{Lapses: s.Lapses, Stability: s.Stability}, last), "dropped %s outranks kept %s", s.CardID, last.CardID)
				}
			}
		}
	})
}

// TestNormalizedDifficulty_Property_ClampedTenth: the result is clamp(d/10, 0, 1)
// and is monotone in d.
func TestNormalizedDifficulty_Property_ClampedTenth(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(t *rapid.T) {
		gen := rapid.OneOf(rapid.Float64Range(-20, 30), rapid.SampledFrom([]float64{0, 1, 10, math.Nextafter(10, 11)}))
		a, b := gen.Draw(t, "a"), gen.Draw(t, "b")
		require.InDelta(t, math.Min(1, math.Max(0, a/10)), normalizedDifficulty(a), 1e-12)
		if a <= b {
			require.LessOrEqual(t, normalizedDifficulty(a), normalizedDifficulty(b))
		}
	})
}
