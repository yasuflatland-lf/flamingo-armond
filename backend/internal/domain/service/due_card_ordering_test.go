package service

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

// dueCard builds a DueCard fixture with a Card carrying the given ID.
func dueCard(id string, state domain.FSRSCardState, due time.Time) domain.DueCard {
	return domain.DueCard{
		Card:  &domain.Card{ID: id},
		State: state,
		Due:   due,
	}
}

func cardIDs(cards []*domain.Card) []string {
	out := make([]string, len(cards))
	for i, card := range cards {
		out[i] = card.ID
	}
	return out
}

func TestOrderingPolicy_Apply_Empty(t *testing.T) {
	t.Parallel()

	got := NewOrderingPolicy().Apply(nil, rand.New(rand.NewSource(42)))
	require.Empty(t, got)

	got = NewOrderingPolicy().Apply([]domain.DueCard{}, rand.New(rand.NewSource(42)))
	require.Empty(t, got)
}

func TestOrderingPolicy_Apply_OnlyNew(t *testing.T) {
	t.Parallel()

	// All distinct Due timestamps so shuffleSameDue is a no-op and the
	// post-partition order is identical to the input order.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("n1", domain.FSRSStateNew, base),
		dueCard("n2", domain.FSRSStateNew, base.Add(time.Minute)),
		dueCard("n3", domain.FSRSStateNew, base.Add(2*time.Minute)),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))

	require.Equal(t, []string{"n1", "n2", "n3"}, cardIDs(got))
}

func TestOrderingPolicy_Apply_OnlyReview(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("r1", domain.FSRSStateReview, base),
		dueCard("r2", domain.FSRSStateLearning, base.Add(time.Minute)),
		dueCard("r3", domain.FSRSStateRelearning, base.Add(2*time.Minute)),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))

	require.Equal(t, []string{"r1", "r2", "r3"}, cardIDs(got))
}

func TestOrderingPolicy_Apply_MixedInterleaveRatio(t *testing.T) {
	t.Parallel()

	// 5 new + 20 review, all with distinct Due timestamps so the shuffle
	// step is a no-op and we can assert the deterministic interleave order.
	// Expected pattern: review-first, ReviewCardRatio (4) reviews then
	// NewCardRatio (1) new, repeating until both buckets empty.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := make([]domain.DueCard, 0, 25)
	for i := 0; i < 20; i++ {
		in = append(in, dueCard(
			fmt.Sprintf("rev-%d", i),
			domain.FSRSStateReview,
			base.Add(time.Duration(i)*time.Minute),
		))
	}
	for i := 0; i < 5; i++ {
		in = append(in, dueCard(
			fmt.Sprintf("new-%d", i),
			domain.FSRSStateNew,
			base.Add(time.Duration(100+i)*time.Minute),
		))
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))

	require.Len(t, got, 25)

	// Assert first 10 cards exactly: 4 reviews, 1 new, 4 reviews, 1 new.
	wantFirst10 := []string{
		"rev-0", "rev-1", "rev-2", "rev-3", "new-0",
		"rev-4", "rev-5", "rev-6", "rev-7", "new-1",
	}
	require.Equal(t, wantFirst10, cardIDs(got)[:10])

	// Full expected order documents the policy end-to-end:
	wantAll := []string{
		"rev-0", "rev-1", "rev-2", "rev-3", "new-0",
		"rev-4", "rev-5", "rev-6", "rev-7", "new-1",
		"rev-8", "rev-9", "rev-10", "rev-11", "new-2",
		"rev-12", "rev-13", "rev-14", "rev-15", "new-3",
		"rev-16", "rev-17", "rev-18", "rev-19", "new-4",
	}
	require.Equal(t, wantAll, cardIDs(got))
}

func TestOrderingPolicy_Apply_SameDueShuffled(t *testing.T) {
	t.Parallel()

	// All four cards share the same Due, so shuffleSameDue MUST permute the
	// run. With rand.NewSource(42), four identical-Due cards [a, b, c, d]
	// shuffle to [c, d, a, b] — see the comment in the file documenting the
	// seed.
	due := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("a", domain.FSRSStateReview, due),
		dueCard("b", domain.FSRSStateReview, due),
		dueCard("c", domain.FSRSStateReview, due),
		dueCard("d", domain.FSRSStateReview, due),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))

	gotIDs := cardIDs(got)
	require.Len(t, gotIDs, 4)
	require.NotEqual(t, []string{"a", "b", "c", "d"}, gotIDs,
		"same-Due run must be shuffled, not left in input order")
	require.Equal(t, []string{"c", "d", "a", "b"}, gotIDs,
		"deterministic shuffle order for seed 42")
}

func TestOrderingPolicy_Apply_SetEquality(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("n1", domain.FSRSStateNew, base),
		dueCard("n2", domain.FSRSStateNew, base.Add(time.Minute)),
		dueCard("r1", domain.FSRSStateReview, base),
		dueCard("r2", domain.FSRSStateReview, base),
		dueCard("r3", domain.FSRSStateLearning, base.Add(2*time.Minute)),
		dueCard("r4", domain.FSRSStateRelearning, base.Add(3*time.Minute)),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))

	require.Len(t, got, len(in), "no cards dropped or duplicated")

	wantPtrs := make(map[*domain.Card]struct{}, len(in))
	for _, d := range in {
		wantPtrs[d.Card] = struct{}{}
	}
	gotPtrs := make(map[*domain.Card]struct{}, len(got))
	for _, c := range got {
		gotPtrs[c] = struct{}{}
	}
	require.Equal(t, wantPtrs, gotPtrs,
		"output Card pointer set must equal input Card pointer set")
}

func TestOrderingPolicy_Apply_PanicsOnNilRng(t *testing.T) {
	t.Parallel()

	require.PanicsWithValue(t,
		"domain/service: OrderingPolicy.Apply requires non-nil rng",
		func() {
			_ = NewOrderingPolicy().Apply(nil, nil)
		},
	)
}
