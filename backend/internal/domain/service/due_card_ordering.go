package service

import (
	"math/rand"

	"backend/internal/domain"
)

// NewCardRatio and ReviewCardRatio define the new/review interleave ratio.
// Today fixed at 1:4 (one new per four reviews) per the 2026-05 product
// decision; revisit if data shows the queue ballooning or starving.
const (
	NewCardRatio    = 1
	ReviewCardRatio = 4
)

// OrderingPolicy applies session-local ordering to due cards.
//
// The zero value is ready to use; the constructor is provided for symmetry
// with other domain services.
type OrderingPolicy struct{}

func NewOrderingPolicy() *OrderingPolicy { return &OrderingPolicy{} }

// Apply orders due cards with a two-step policy:
//
//  1. Within each partition (new / review), cards sharing the same Due
//     timestamp are shuffled using rng so consecutive sessions do not see
//     the same first-N order.
//  2. New and review cards are interleaved at NewCardRatio:ReviewCardRatio
//     with review-first emission. When one bucket empties, the remaining
//     cards from the other bucket are appended in their post-shuffle order.
//
// The caller is responsible for truncating to a per-session limit. Apply
// returns every Card supplied in due so the caller can decide whether to
// keep more than the originally requested limit for tomorrow's queue.
//
// rng must be non-nil. Tests inject a seeded *rand.Rand for deterministic
// order; production constructs one per session.
func (p *OrderingPolicy) Apply(due []domain.DueCard, rng *rand.Rand) []*domain.Card {
	if rng == nil {
		panic("domain/service: OrderingPolicy.Apply requires non-nil rng")
	}
	newCards, reviewCards := partition(due)
	shuffleSameDue(newCards, rng)
	shuffleSameDue(reviewCards, rng)
	return interleave(newCards, reviewCards, NewCardRatio, ReviewCardRatio)
}

// partition splits due into new (FSRSStateNew) vs review (everything else),
// preserving the input order. The repository pre-sorts by Due ASC, id ASC.
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

// shuffleSameDue shuffles contiguous same-Due runs in place using rng.
// Inputs are pre-sorted by Due ASC so a single linear pass detects each run.
// Single-card runs are untouched.
func shuffleSameDue(cards []domain.DueCard, rng *rand.Rand) {
	start := 0
	for i := 1; i <= len(cards); i++ {
		if i == len(cards) || !cards[i].Due.Equal(cards[start].Due) {
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
