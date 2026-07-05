package service

import (
	"fmt"
	"math/rand"

	"backend/internal/domain"
)

// OrderingPolicy applies session-local ordering to due cards.
//
// The zero value is ready to use; the constructor is provided for symmetry
// with other domain services.
type OrderingPolicy struct{}

func NewOrderingPolicy() *OrderingPolicy { return &OrderingPolicy{} }

// Apply orders due cards with a two-step policy:
//
//  1. The new partition is fully shuffled — the repository samples WHICH new
//     cards enter the batch (uniformly, via SQL random()); this shuffle
//     randomises their arrangement deterministically under an injected rng.
//     The review partition is shuffled within same-phase runs: the repository
//     pre-sorts learning-phase rows (Learning/Relearning) ahead of Review
//     rows, and shuffling never crosses that boundary, so a Review-state
//     filler can never displace a learning-phase card from the review slots.
//  2. New and review cards are interleaved at the caller-supplied ratio
//     (ratio.NewShare new per ratio.ReviewShare review) with review-first
//     emission. When one bucket empties, the remaining cards from the other
//     bucket are appended in their post-shuffle order.
//
// The caller is responsible for truncating to a per-session limit. Apply
// returns all cards from due without imposing a length cap; the input slice
// is not modified, as partition produces fresh slices before shuffling.
//
// rng must be non-nil. Tests inject a seeded *rand.Rand for deterministic
// order; production constructs one per session. ratio is the per-user
// new-vs-review interleave ratio; its VO invariants guarantee both shares are
// >= 1, so interleave's positive-ratio guard is satisfied by construction.
func (p *OrderingPolicy) Apply(due []domain.DueCard, rng *rand.Rand, ratio domain.NewCardRatio) []*domain.Card {
	if rng == nil {
		panic("domain/service: OrderingPolicy.Apply requires non-nil rng")
	}
	for _, d := range due {
		if d.Card == nil {
			panic("domain/service: OrderingPolicy.Apply: DueCard.Card must not be nil")
		}
	}
	newCards, reviewCards := partition(due)
	rng.Shuffle(len(newCards), func(a, b int) {
		newCards[a], newCards[b] = newCards[b], newCards[a]
	})
	shuffleWithinPhase(reviewCards, rng)
	return interleave(newCards, reviewCards, ratio.NewShare(), ratio.ReviewShare())
}

// partition splits due into new (FSRSStateNew) vs review (everything else),
// preserving the input order. The repository pre-sorts review rows
// learning-phase first, then random() within each phase; new rows arrive in
// random() sample order.
func partition(due []domain.DueCard) (newC, reviewC []domain.DueCard) {
	for _, d := range due {
		if d.State == domain.FSRSStateNew {
			newC = append(newC, d)
		} else {
			reviewC = append(reviewC, d)
		}
	}
	return
}

// shuffleWithinPhase shuffles contiguous same-phase runs in place using rng.
// The repository pre-sorts review cards learning-phase first (Learning and
// Relearning before Review), so a single linear pass detects each phase run.
// Scoping the shuffle to a run preserves the phase priority: a Review-state
// filler card can never move ahead of a learning-phase card.
func shuffleWithinPhase(cards []domain.DueCard, rng *rand.Rand) {
	start := 0
	// Loop runs through len(cards) inclusive so the trailing run is flushed
	// without a tail handler.
	for i := 1; i <= len(cards); i++ {
		if i == len(cards) || cards[i].State.IsLearningPhase() != cards[start].State.IsLearningPhase() {
			if i-start > 1 {
				run := cards[start:i]
				rng.Shuffle(len(run), func(a, b int) { run[a], run[b] = run[b], run[a] })
			}
			start = i
		}
	}
}

// interleave emits rRatio review cards then nRatio new cards in a loop until
// one bucket empties, then appends the remaining bucket in its current order.
// Returns the flattened []*Card extracted from DueCard.Card.
func interleave(newC, reviewC []domain.DueCard, nRatio, rRatio int) []*domain.Card {
	if nRatio <= 0 || rRatio <= 0 {
		panic(fmt.Sprintf("domain/service: interleave requires positive ratios, got nRatio=%d rRatio=%d", nRatio, rRatio))
	}
	out := make([]*domain.Card, 0, len(newC)+len(reviewC))
	i, j := 0, 0
	for i < len(newC) && j < len(reviewC) {
		for k := 0; k < rRatio && j < len(reviewC); k++ {
			out = append(out, reviewC[j].Card)
			j++
		}
		for k := 0; k < nRatio && i < len(newC); k++ {
			out = append(out, newC[i].Card)
			i++
		}
	}
	for ; j < len(reviewC); j++ {
		out = append(out, reviewC[j].Card)
	}
	for ; i < len(newC); i++ {
		out = append(out, newC[i].Card)
	}
	return out
}
