package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

func fsrsStat(card, cardgroup string, phase domain.FSRSPhase, stability float64, lapses int) domain.FSRSStat {
	return domain.FSRSStat{CardID: card, CardgroupID: cardgroup, Phase: phase, Stability: stability, Lapses: lapses}
}

func strugglingIDs(cards []StrugglingCard) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.CardID)
	}
	return out
}

// TestAggregateMastery pins the accumulation invariant that defines "mastered":
// every studied stat lands in exactly one of the three disjoint tiers (so
// InProgress + Learned + Mature == TotalStudied), and one DeckMastery is emitted
// per owned deck in deckCardTotals — CardgroupID-sorted, with the learned/mature
// split disjoint and a zero-studied deck still emitted with learned=mature=0.
func TestAggregateMastery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		stats       []domain.FSRSStat
		totals      map[string]int
		wantMastery MasteryBreakdown
		wantDecks   []DeckMastery
	}{
		{
			name:        "empty history yields zero breakdown and no decks",
			stats:       nil,
			totals:      map[string]int{},
			wantMastery: MasteryBreakdown{},
			wantDecks:   []DeckMastery{},
		},
		{
			name: "three disjoint tiers across two decks, deterministic sort, empty deck emitted",
			stats: []domain.FSRSStat{
				fsrsStat("a", "cg1", domain.FSRSPhaseReview, 30, 0),                         // mature
				fsrsStat("b", "cg1", domain.FSRSPhaseReview, 10, 0),                         // learned
				fsrsStat("c", "cg1", domain.FSRSPhaseNew, 0, 0),                             // in progress
				fsrsStat("d", "cg2", domain.FSRSPhaseReview, domain.MatureStabilityDays, 0), // mature (boundary, >=)
				fsrsStat("e", "cg2", domain.FSRSPhaseLearning, 0, 0),                        // in progress
			},
			totals:      map[string]int{"cg1": 5, "cg2": 3, "cg3": 2},
			wantMastery: MasteryBreakdown{InProgress: 2, Learned: 1, Mature: 2, TotalStudied: 5},
			wantDecks: []DeckMastery{
				{CardgroupID: "cg1", TotalCards: 5, LearnedCards: 1, MatureCards: 1},
				{CardgroupID: "cg2", TotalCards: 3, LearnedCards: 0, MatureCards: 1},
				{CardgroupID: "cg3", TotalCards: 2, LearnedCards: 0, MatureCards: 0},
			},
		},
		{
			name: "just below the mature boundary is Learned, not Mature",
			stats: []domain.FSRSStat{
				fsrsStat("b", "cg1", domain.FSRSPhaseReview, domain.MatureStabilityDays-0.01, 0),
			},
			totals:      map[string]int{"cg1": 1},
			wantMastery: MasteryBreakdown{Learned: 1, TotalStudied: 1},
			wantDecks:   []DeckMastery{{CardgroupID: "cg1", TotalCards: 1, LearnedCards: 1, MatureCards: 0}},
		},
		{
			name: "studied card whose deck is absent from totals counts globally but has no deck row",
			stats: []domain.FSRSStat{
				fsrsStat("x", "cgX", domain.FSRSPhaseReview, 30, 0),
			},
			totals:      map[string]int{},
			wantMastery: MasteryBreakdown{Mature: 1, TotalStudied: 1},
			wantDecks:   []DeckMastery{},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mastery, decks, err := AggregateMastery(tc.stats, tc.totals)
			require.NoError(t, err)
			require.Equal(t, tc.wantMastery, mastery)
			require.Equal(t, tc.wantDecks, decks)

			// The core accumulation invariant: the three tiers are disjoint and
			// cover every studied stat.
			require.Equal(t, mastery.TotalStudied,
				mastery.InProgress+mastery.Learned+mastery.Mature,
				"tiers must sum to TotalStudied")
			require.Equal(t, len(tc.stats), mastery.TotalStudied)
			for _, d := range decks {
				require.LessOrEqual(t, d.LearnedCards+d.MatureCards, d.TotalCards,
					"acquired never exceeds the denominator for deck %s", d.CardgroupID)
			}
		})
	}
}

// TestTopStruggling pins the "struggling" definition: a stat with no lapse is
// never struggling, and among lapsed stats the ranking is (Lapses desc,
// Stability asc).
func TestTopStruggling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		stats   []domain.FSRSStat
		limit   int
		wantIDs []string
	}{
		{
			name: "excludes zero-lapse stats and orders by lapses then stability",
			stats: []domain.FSRSStat{
				fsrsStat("no-lapse", "cg1", domain.FSRSPhaseReview, 1, 0), // filtered out (Lapses == 0)
				fsrsStat("one-lapse", "cg1", domain.FSRSPhaseReview, 50, 1),
				fsrsStat("five-a", "cg1", domain.FSRSPhaseReview, 8, 5),
				fsrsStat("five-b", "cg1", domain.FSRSPhaseReview, 3, 5), // same lapses, lower stability -> first
			},
			limit:   10,
			wantIDs: []string{"five-b", "five-a", "one-lapse"},
		},
		{
			name: "no lapsed stats yields an empty (non-nil) slice",
			stats: []domain.FSRSStat{
				fsrsStat("no-lapse", "cg1", domain.FSRSPhaseReview, 1, 0),
			},
			limit:   10,
			wantIDs: []string{},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := TopStruggling(tc.stats, tc.limit)
			require.NotNil(t, got, "the struggling set is always a non-nil slice")
			assert.Equal(t, tc.wantIDs, strugglingIDs(got))
			for _, c := range got {
				assert.GreaterOrEqual(t, c.Lapses, 1)
			}
		})
	}
}

// TestTopStruggling_CapsAtLimit verifies the result is truncated to the limit,
// keeping the highest-lapse stats.
func TestTopStruggling_CapsAtLimit(t *testing.T) {
	t.Parallel()

	const limit = 10
	stats := make([]domain.FSRSStat, 0, 15)
	for i := 0; i < 15; i++ {
		stats = append(stats, fsrsStat(fmt.Sprintf("card-%02d", i), "cg1", domain.FSRSPhaseReview, 1, i+1))
	}

	got := TopStruggling(stats, limit)

	require.Len(t, got, limit, "capped at limit")
	assert.Equal(t, 15, got[0].Lapses, "highest lapses first")
	assert.Equal(t, 6, got[limit-1].Lapses, "lowest surviving lapses at the cap boundary")
}
