package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

func fsrsStat(card, cardgroup string, phase domain.FSRSPhase, stability float64, lapses int) domain.FSRSStat {
	return domain.FSRSStat{CardID: card, CardgroupID: cardgroup, Phase: phase, Stability: stability, Lapses: lapses}
}

// TestAggregateMastery is the worked example of the partition law
// TestAggregateMastery_Property_PartitionsStats checks in general: three
// disjoint tiers across two decks, the >= mature boundary, and a zero-studied
// deck still emitted with learned=mature=0.
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
