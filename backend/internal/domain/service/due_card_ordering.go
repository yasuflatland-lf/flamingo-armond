package service

import (
	"math/rand"
	"sort"

	"backend/internal/domain"
)

// OrderingPolicy applies the learning queue ordering rule:
// due ASC, with cards sharing the same due timestamp shuffled by the caller's
// random source. Apply is pure: it never mutates the input slice.
type OrderingPolicy struct{}

func NewOrderingPolicy() *OrderingPolicy { return &OrderingPolicy{} }

func (p *OrderingPolicy) Apply(cards []*domain.Card, r *rand.Rand) []*domain.Card {
	out := append([]*domain.Card(nil), cards...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].FSRS.Due.Before(out[j].FSRS.Due)
	})
	if r == nil {
		return out
	}
	for start := 0; start < len(out); {
		end := start + 1
		for end < len(out) && out[end].FSRS.Due.Equal(out[start].FSRS.Due) {
			end++
		}
		if end-start > 1 {
			run := out[start:end]
			r.Shuffle(len(run), func(i, j int) { run[i], run[j] = run[j], run[i] })
		}
		start = end
	}
	return out
}
