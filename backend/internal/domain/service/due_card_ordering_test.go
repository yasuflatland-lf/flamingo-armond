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
// The Card's Position defaults to 0.
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

	got := NewOrderingPolicy().Apply(nil, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)
	require.Empty(t, got)

	got = NewOrderingPolicy().Apply([]domain.DueCard{}, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)
	require.Empty(t, got)
}

func TestOrderingPolicy_Apply_OnlyNew_AlwaysShuffled(t *testing.T) {
	t.Parallel()

	// Four new cards with DISTINCT positions: under the old tie-scoped policy
	// these were never reshuffled; the discovery-first policy always shuffles
	// the whole new partition. With rand.NewSource(42) a 4-element shuffle
	// permutes [a,b,c,d] -> [c,d,a,b].
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("n1", domain.FSRSStateNew, base),
		dueCard("n2", domain.FSRSStateNew, base.Add(time.Minute)),
		dueCard("n3", domain.FSRSStateNew, base.Add(2*time.Minute)),
		dueCard("n4", domain.FSRSStateNew, base.Add(3*time.Minute)),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)

	require.Equal(t, []string{"n3", "n4", "n1", "n2"}, cardIDs(got),
		"deterministic full shuffle for seed 42")
}

func TestOrderingPolicy_Apply_OnlyReview_PhaseRunsShuffledIndependently(t *testing.T) {
	t.Parallel()

	// Repository contract: learning-phase rows arrive BEFORE Review rows.
	// Two learning-phase + two Review cards. Each phase run is shuffled
	// independently and the phase boundary is never crossed. With seed 42
	// (new partition is empty, so the first rng consumption is the learning
	// run) each 2-element run swaps.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("l1", domain.FSRSStateLearning, base),
		dueCard("l2", domain.FSRSStateRelearning, base.Add(time.Minute)),
		dueCard("r1", domain.FSRSStateReview, base.Add(2*time.Minute)),
		dueCard("r2", domain.FSRSStateReview, base.Add(3*time.Minute)),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)

	require.Equal(t, []string{"l2", "l1", "r2", "r1"}, cardIDs(got),
		"each phase run shuffles independently; learning phase stays first")
}

func TestOrderingPolicy_Apply_PhaseBoundaryHoldsAcrossSeeds(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("l1", domain.FSRSStateLearning, base),
		dueCard("l2", domain.FSRSStateLearning, base.Add(time.Minute)),
		dueCard("l3", domain.FSRSStateRelearning, base.Add(2*time.Minute)),
		dueCard("r1", domain.FSRSStateReview, base.Add(3*time.Minute)),
		dueCard("r2", domain.FSRSStateReview, base.Add(4*time.Minute)),
	}
	learning := map[string]bool{"l1": true, "l2": true, "l3": true}

	for _, seed := range []int64{1, 7, 42, 99} {
		got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(seed)), domain.DefaultNewCardRatio)
		ids := cardIDs(got)
		require.Len(t, ids, 5)
		for i, id := range ids[:3] {
			require.True(t, learning[id],
				"seed %d: slot %d must be learning-phase, got %q", seed, i, id)
		}
	}
}

func TestOrderingPolicy_Apply_MixedCompositionSlots(t *testing.T) {
	t.Parallel()

	// 5 review + 20 new. Interleave emits 1 review then 4 new per cycle, so
	// review cards occupy exactly slots 0, 5, 10, 15, 20 regardless of how
	// the shuffles permute identities WITHIN each partition.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := make([]domain.DueCard, 0, 25)
	reviewSet := make(map[string]bool, 5)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("rev-%d", i)
		reviewSet[id] = true
		in = append(in, dueCard(id, domain.FSRSStateLearning, base.Add(time.Duration(i)*time.Minute)))
	}
	for i := 0; i < 20; i++ {
		in = append(in, dueCard(fmt.Sprintf("new-%d", i), domain.FSRSStateNew, base.Add(time.Duration(100+i)*time.Minute)))
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)
	require.Len(t, got, 25)

	for i, c := range got {
		if i%5 == 0 {
			require.True(t, reviewSet[c.ID], "slot %d must be a review card, got %q", i, c.ID)
		} else {
			require.False(t, reviewSet[c.ID], "slot %d must be a new card, got %q", i, c.ID)
		}
	}
}

func TestOrderingPolicy_Apply_NonDefaultRatioInterleavesOneToOne(t *testing.T) {
	t.Parallel()

	// 3 new + 3 review. The default 4/5 ratio would emit [R,N,N,N,R,R]; a 1/2
	// ratio (new share 1, review share 1) interleaves 1:1 review-first, so
	// review cards land in the even slots and new cards in the odd slots. The
	// distinct slot composition proves the caller-supplied ratio reaches Apply.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("n1", domain.FSRSStateNew, base),
		dueCard("n2", domain.FSRSStateNew, base.Add(time.Minute)),
		dueCard("n3", domain.FSRSStateNew, base.Add(2*time.Minute)),
		dueCard("r1", domain.FSRSStateReview, base.Add(3*time.Minute)),
		dueCard("r2", domain.FSRSStateReview, base.Add(4*time.Minute)),
		dueCard("r3", domain.FSRSStateReview, base.Add(5*time.Minute)),
	}
	reviewSet := map[string]bool{"r1": true, "r2": true, "r3": true}

	ratio, err := domain.ParseNewCardRatio(1, 2)
	require.NoError(t, err)

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), ratio)
	require.Len(t, got, 6)

	for i, c := range got {
		if i%2 == 0 {
			require.True(t, reviewSet[c.ID], "slot %d must be a review card, got %q", i, c.ID)
		} else {
			require.False(t, reviewSet[c.ID], "slot %d must be a new card, got %q", i, c.ID)
		}
	}
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

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)

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
			_ = NewOrderingPolicy().Apply(nil, nil, domain.DefaultNewCardRatio)
		},
	)
}

func TestOrderingPolicy_Apply_PanicsOnNilCard(t *testing.T) {
	t.Parallel()

	due := []domain.DueCard{
		{Card: nil, State: domain.FSRSStateNew},
	}
	require.PanicsWithValue(t,
		"domain/service: OrderingPolicy.Apply: DueCard.Card must not be nil",
		func() {
			_ = NewOrderingPolicy().Apply(due, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)
		},
	)
}

// interleave is exercised directly for the trailing-append paths so the
// assertions stay deterministic without depending on shuffle permutations.
func TestInterleave_TrailingReviewAppend(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	newC := []domain.DueCard{dueCard("new-0", domain.FSRSStateNew, base)}
	reviewC := make([]domain.DueCard, 0, 7)
	for i := 0; i < 7; i++ {
		reviewC = append(reviewC, dueCard(fmt.Sprintf("rev-%d", i), domain.FSRSStateReview, base.Add(time.Duration(i)*time.Minute)))
	}

	got := interleave(newC, reviewC, 4, 1)

	// Cycle 1 emits rev-0 then new-0 (new bucket exhausts the 4-slot k-loop
	// at 1 card); the outer loop exits and the trailing-review path appends
	// rev-1..rev-6.
	want := []string{"rev-0", "new-0", "rev-1", "rev-2", "rev-3", "rev-4", "rev-5", "rev-6"}
	require.Equal(t, want, cardIDs(got))
}

func TestInterleave_TrailingNewAppend(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	newC := make([]domain.DueCard, 0, 10)
	for i := 0; i < 10; i++ {
		newC = append(newC, dueCard(fmt.Sprintf("new-%d", i), domain.FSRSStateNew, base.Add(time.Duration(100+i)*time.Minute)))
	}
	reviewC := []domain.DueCard{
		dueCard("rev-0", domain.FSRSStateReview, base),
		dueCard("rev-1", domain.FSRSStateReview, base.Add(time.Minute)),
	}

	got := interleave(newC, reviewC, 4, 1)

	// Cycle 1: rev-0, new-0..new-3. Cycle 2: rev-1, new-4..new-7. Review
	// bucket is now empty so the outer loop exits; trailing-new appends
	// new-8, new-9.
	want := []string{
		"rev-0", "new-0", "new-1", "new-2", "new-3",
		"rev-1", "new-4", "new-5", "new-6", "new-7",
		"new-8", "new-9",
	}
	require.Equal(t, want, cardIDs(got))
}

func TestInterleave_PanicsOnNonPositiveRatios(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	newC := []domain.DueCard{dueCard("new-0", domain.FSRSStateNew, base)}
	reviewC := []domain.DueCard{dueCard("rev-0", domain.FSRSStateReview, base)}

	require.Panics(t, func() { interleave(newC, reviewC, 0, 1) },
		"zero nRatio must panic")
	require.Panics(t, func() { interleave(newC, reviewC, 4, 0) },
		"zero rRatio must panic")
}
