package service

import (
	"math/rand"

	"backend/internal/domain"
)

// OrderingPolicy preserves repository ordering for the learning queue.
// Per-user due ordering now lives in SQL so the domain Card does not carry
// viewer-specific FSRS state. Apply is pure: it never mutates the input slice.
type OrderingPolicy struct{}

func NewOrderingPolicy() *OrderingPolicy { return &OrderingPolicy{} }

func (p *OrderingPolicy) Apply(cards []*domain.Card, r *rand.Rand) []*domain.Card {
	_ = r
	return append([]*domain.Card(nil), cards...)
}
