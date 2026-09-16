package service

import (
	"fmt"

	"backend/internal/domain"
)

// OrderingPolicy applies session-local ordering to due cards.
//
// The zero value is ready to use; the constructor is provided for symmetry
// with other domain services.
type OrderingPolicy struct{}

func NewOrderingPolicy() *OrderingPolicy { return &OrderingPolicy{} }

// Apply partitions by Phase == FSRSPhaseNew and interleaves at the ratio by largest remainder.
// It preserves input order within each partition: selection and per-kind order belong to the repository.
// The input is unchanged; the caller truncates the returned cards to its session limit.
func (p *OrderingPolicy) Apply(due []domain.DueCard, ratio domain.NewCardRatio) []*domain.Card {
	for _, d := range due {
		if d.Card == nil {
			panic("domain/service: OrderingPolicy.Apply: DueCard.Card must not be nil")
		}
	}
	newCards, reviewCards := partition(due)
	return interleave(newCards, reviewCards, ratio.NewShare(), ratio.ReviewShare())
}

// partition splits due into new (FSRSPhaseNew) vs review (everything else),
// preserving the input order. New rows arrive newest-added first (see
// repository.findDueCardsOn); review rows arrive by descending retrievability.
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
