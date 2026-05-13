package service

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

func TestOrderingPolicyApplyPreservesRepositoryOrder(t *testing.T) {
	t.Parallel()

	cards := []*domain.Card{
		testOrderingCard("repo-first"),
		testOrderingCard("repo-second"),
		testOrderingCard("repo-third"),
	}
	before := append([]*domain.Card(nil), cards...)

	got := NewOrderingPolicy().Apply(cards, rand.New(rand.NewSource(42)))

	require.Equal(t, []string{"repo-first", "repo-second", "repo-third"}, cardIDs(got))
	require.Equal(t, before, cards, "Apply must not mutate the input slice")
}

func TestOrderingPolicyApplyReturnsIndependentSlice(t *testing.T) {
	t.Parallel()

	cards := []*domain.Card{
		testOrderingCard("a"),
		testOrderingCard("b"),
	}

	got := NewOrderingPolicy().Apply(cards, nil)
	got[0], got[1] = got[1], got[0]

	require.Equal(t, []string{"a", "b"}, cardIDs(cards))
	require.Equal(t, []string{"b", "a"}, cardIDs(got))
}

func testOrderingCard(id string) *domain.Card {
	return &domain.Card{
		ID: id,
	}
}

func cardIDs(cards []*domain.Card) []string {
	out := make([]string, len(cards))
	for i, card := range cards {
		out[i] = card.ID
	}
	return out
}
