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
//     The review partition is stable-partitioned by Rescue (rescue first)
//     and each sub-partition is shuffled independently, so a filler can
//     never displace a rescue card from the review slots regardless of
//     input order.
//  2. New and review cards are interleaved at the caller-supplied ratio
//     (ratio.NewShare new per ratio.ReviewShare review) by largest-remainder
//     distribution: each slot goes to whichever bucket is furthest behind its
//     share, so the ratio holds on every prefix and not only on whole cycles.
//     When one bucket empties, the remaining cards from the other bucket are
//     appended in their post-shuffle order.
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
	shuffleRescueFirst(reviewCards, rng)
	return interleave(newCards, reviewCards, ratio.NewShare(), ratio.ReviewShare())
}

// partition splits due into new (FSRSPhaseNew) vs review (everything else),
// preserving the input order. New rows arrive in random() sample order.
func partition(due []domain.DueCard) (newC, reviewC []domain.DueCard) {
	for _, d := range due {
		if d.Phase == domain.FSRSPhaseNew {
			newC = append(newC, d)
		} else {
			reviewC = append(reviewC, d)
		}
	}
	return
}

// shuffleRescueFirst stable-partitions cards by Rescue (rescue first) and
// shuffles each partition independently in place, so a filler card can never
// move ahead of a rescue card regardless of input order. Rescue is shuffled
// before filler to keep rng consumption stable for already-partitioned input.
func shuffleRescueFirst(cards []domain.DueCard, rng *rand.Rand) {
	rescue := make([]domain.DueCard, 0, len(cards))
	filler := make([]domain.DueCard, 0, len(cards))
	for _, c := range cards {
		if c.Rescue {
			rescue = append(rescue, c)
		} else {
			filler = append(filler, c)
		}
	}
	rng.Shuffle(len(rescue), func(a, b int) { rescue[a], rescue[b] = rescue[b], rescue[a] })
	rng.Shuffle(len(filler), func(a, b int) { filler[a], filler[b] = filler[b], filler[a] })
	copy(cards, rescue)
	copy(cards[len(rescue):], filler)
}

// interleave fills one output slot at a time from whichever bucket is furthest
// behind its share (largest remainder), so the ratio holds on every prefix and
// not only on whole cycles. When one bucket empties, the remainder of the other
// is appended in its current order. Returns the flattened []*Card extracted
// from DueCard.Card.
func interleave(newC, reviewC []domain.DueCard, nRatio, rRatio int) []*domain.Card {
	if nRatio <= 0 || rRatio <= 0 {
		panic(fmt.Sprintf("domain/service: interleave requires positive ratios, got nRatio=%d rRatio=%d", nRatio, rRatio))
	}
	den := nRatio + rRatio
	out := make([]*domain.Card, 0, len(newC)+len(reviewC))
	i, j := 0, 0
	for i < len(newC) && j < len(reviewC) {
		// An exact half-card tie goes to the new bucket, not review: a
		// review-biased tie-break leaves a one-card session at ratio 1/2 with
		// no new card at all.
		if targetNewCount(len(out)+1, nRatio, den) > i {
			out = append(out, newC[i].Card)
			i++
		} else {
			out = append(out, reviewC[j].Card)
			j++
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

// targetNewCount is round(slot*nRatio/den) in integer arithmetic, halves up.
// Float division is rejected: a float round would tie-break on representation
// error, and 2*slot*nRatio never approaches the int range here.
func targetNewCount(slot, nRatio, den int) int {
	return (2*slot*nRatio + den) / (2 * den)
}
