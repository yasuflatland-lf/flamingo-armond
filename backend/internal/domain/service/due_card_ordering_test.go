package service

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

func TestOrderingPolicyApplyContract(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	cards := []*domain.Card{
		testOrderingCard("later-a", base.Add(2*time.Hour)),
		testOrderingCard("same-b", base),
		testOrderingCard("earlier", base.Add(-time.Hour)),
		testOrderingCard("same-a", base),
		testOrderingCard("later-b", base.Add(2*time.Hour)),
	}
	before := append([]*domain.Card(nil), cards...)

	got := NewOrderingPolicy().Apply(cards, rand.New(rand.NewSource(42)))

	require.Len(t, got, len(cards))
	require.ElementsMatch(t, cardIDs(cards), cardIDs(got))
	require.Equal(t, before, cards, "Apply must not mutate the input slice")
	for i := 1; i < len(got); i++ {
		require.False(t, got[i].FSRS.Due.Before(got[i-1].FSRS.Due))
	}
	require.Equal(t, "earlier", got[0].ID)
	require.ElementsMatch(t, []string{"same-a", "same-b"}, cardIDs(got[1:3]))
	require.ElementsMatch(t, []string{"later-a", "later-b"}, cardIDs(got[3:]))
}

func TestOrderingPolicyApplyDeterministicForSameSeed(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	cards := []*domain.Card{
		testOrderingCard("a", due),
		testOrderingCard("b", due),
		testOrderingCard("c", due),
	}
	policy := NewOrderingPolicy()

	got1 := policy.Apply(cards, rand.New(rand.NewSource(42)))
	got2 := policy.Apply(cards, rand.New(rand.NewSource(42)))

	require.Equal(t, cardIDs(got1), cardIDs(got2))
}

func TestOrderingPolicyApplyShufflesSameDueRun(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 5, 13, 9, 0, 0, 0, time.UTC)
	cards := []*domain.Card{
		testOrderingCard("a", due),
		testOrderingCard("b", due),
		testOrderingCard("c", due),
	}
	seen := map[string]struct{}{}
	policy := NewOrderingPolicy()

	for seed := int64(1); seed <= 10; seed++ {
		got := policy.Apply(cards, rand.New(rand.NewSource(seed)))
		seen[key(got)] = struct{}{}
	}

	require.GreaterOrEqual(t, len(seen), 2)
}

func testOrderingCard(id string, due time.Time) *domain.Card {
	return &domain.Card{
		ID:   id,
		FSRS: domain.FSRSState{Due: due},
	}
}

func cardIDs(cards []*domain.Card) []string {
	out := make([]string, len(cards))
	for i, card := range cards {
		out[i] = card.ID
	}
	return out
}

func key(cards []*domain.Card) string {
	out := ""
	for _, card := range cards {
		out += card.ID
	}
	return out
}
