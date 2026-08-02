package service

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"backend/internal/domain"
)

// dueCard builds a DueCard fixture with a Card carrying the given ID.
// The Card's Position defaults to 0.
func dueCard(id string, state domain.FSRSPhase, due time.Time) domain.DueCard {
	return domain.DueCard{
		Card:  &domain.Card{ID: id},
		Phase: state,
		Due:   due,
	}
}

func rescueDueCard(id string, state domain.FSRSPhase, due time.Time) domain.DueCard {
	card := dueCard(id, state, due)
	card.Rescue = true
	return card
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
		dueCard("n1", domain.FSRSPhaseNew, base),
		dueCard("n2", domain.FSRSPhaseNew, base.Add(time.Minute)),
		dueCard("n3", domain.FSRSPhaseNew, base.Add(2*time.Minute)),
		dueCard("n4", domain.FSRSPhaseNew, base.Add(3*time.Minute)),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)

	require.Equal(t, []string{"n3", "n4", "n1", "n2"}, cardIDs(got),
		"deterministic full shuffle for seed 42")
}

func TestOrderingPolicy_Apply_OnlyReview_BandRunsShuffledIndependently(t *testing.T) {
	t.Parallel()

	// Repository contract: rescue rows arrive before filler rows.
	// Two rescue + two filler cards. Each band run is shuffled independently
	// and the band boundary is never crossed. With seed 42
	// (new partition is empty, so the first rng consumption is the rescue
	// run) each 2-element run swaps.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		rescueDueCard("rescue-1", domain.FSRSPhaseReview, base),
		rescueDueCard("rescue-2", domain.FSRSPhaseReview, base.Add(time.Minute)),
		dueCard("filler-1", domain.FSRSPhaseReview, base.Add(2*time.Minute)),
		dueCard("filler-2", domain.FSRSPhaseReview, base.Add(3*time.Minute)),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)

	require.Equal(t, []string{"rescue-2", "rescue-1", "filler-2", "filler-1"}, cardIDs(got),
		"each band run shuffles independently; rescue band stays first")
}

func TestOrderingPolicy_Apply_RescueBoundaryHoldsAcrossSeeds(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		rescueDueCard("rescue-1", domain.FSRSPhaseReview, base),
		rescueDueCard("rescue-2", domain.FSRSPhaseReview, base.Add(time.Minute)),
		rescueDueCard("rescue-3", domain.FSRSPhaseReview, base.Add(2*time.Minute)),
		dueCard("filler-1", domain.FSRSPhaseReview, base.Add(3*time.Minute)),
		dueCard("filler-2", domain.FSRSPhaseReview, base.Add(4*time.Minute)),
	}
	rescue := map[string]bool{"rescue-1": true, "rescue-2": true, "rescue-3": true}

	for _, seed := range []int64{1, 7, 42, 99} {
		got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(seed)), domain.DefaultNewCardRatio)
		ids := cardIDs(got)
		require.Len(t, ids, 5)
		for i, id := range ids[:3] {
			require.True(t, rescue[id],
				"seed %d: slot %d must be rescue, got %q", seed, i, id)
		}
	}
}

func TestOrderingPolicy_Apply_MixedCompositionSlots(t *testing.T) {
	t.Parallel()

	// 5 review + 20 new. Largest-remainder distribution at 4/5 hands every
	// fifth slot from index 2 to the review bucket, so review cards occupy
	// exactly slots 2, 7, 12, 17, 22 regardless of how the shuffles permute
	// identities WITHIN each partition.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := make([]domain.DueCard, 0, 25)
	reviewSet := make(map[string]bool, 5)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("rev-%d", i)
		reviewSet[id] = true
		in = append(in, dueCard(id, domain.FSRSPhaseLearning, base.Add(time.Duration(i)*time.Minute)))
	}
	for i := 0; i < 20; i++ {
		in = append(in, dueCard(fmt.Sprintf("new-%d", i), domain.FSRSPhaseNew, base.Add(time.Duration(100+i)*time.Minute)))
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)
	require.Len(t, got, 25)

	for i, c := range got {
		if i%5 == 2 {
			require.True(t, reviewSet[c.ID], "slot %d must be a review card, got %q", i, c.ID)
		} else {
			require.False(t, reviewSet[c.ID], "slot %d must be a new card, got %q", i, c.ID)
		}
	}
}

func TestOrderingPolicy_Apply_MixedNewRescueAndFillerPreservesReviewBandPriority(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		rescueDueCard("rescue-1", domain.FSRSPhaseReview, base),
		rescueDueCard("rescue-2", domain.FSRSPhaseReview, base.Add(time.Minute)),
		dueCard("filler-1", domain.FSRSPhaseReview, base.Add(2*time.Minute)),
		dueCard("filler-2", domain.FSRSPhaseReview, base.Add(3*time.Minute)),
	}
	for i := 0; i < 12; i++ {
		in = append(in, dueCard(fmt.Sprintf("new-%d", i), domain.FSRSPhaseNew, base.Add(time.Duration(100+i)*time.Minute)))
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), domain.DefaultNewCardRatio)
	require.Len(t, got, len(in))

	rescueIDs := map[string]bool{"rescue-1": true, "rescue-2": true}
	fillerIDs := map[string]bool{"filler-1": true, "filler-2": true}
	seenFiller := false
	for _, card := range got {
		if fillerIDs[card.ID] {
			seenFiller = true
		}
		if rescueIDs[card.ID] {
			require.False(t, seenFiller, "rescue card %q appeared after a filler card", card.ID)
		}
	}
}

func TestOrderingPolicy_Apply_InterleavedRescueFillerInput_RescueStillFirst(t *testing.T) {
	t.Parallel()

	// Deliberately interleaved, filler-leading review input: the policy must
	// enforce rescue-before-filler itself, without relying on the repository
	// emitting rescue rows ahead of filler rows.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("filler-1", domain.FSRSPhaseReview, base),
		rescueDueCard("rescue-1", domain.FSRSPhaseReview, base.Add(time.Minute)),
		dueCard("filler-2", domain.FSRSPhaseReview, base.Add(2*time.Minute)),
		rescueDueCard("rescue-2", domain.FSRSPhaseReview, base.Add(3*time.Minute)),
	}
	rescueIDs := map[string]bool{"rescue-1": true, "rescue-2": true}
	fillerIDs := map[string]bool{"filler-1": true, "filler-2": true}

	for _, seed := range []int64{1, 7, 42, 99} {
		got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(seed)), domain.DefaultNewCardRatio)
		require.Len(t, got, len(in))

		seenFiller := false
		for _, card := range got {
			if fillerIDs[card.ID] {
				seenFiller = true
			}
			if rescueIDs[card.ID] {
				require.False(t, seenFiller,
					"seed %d: rescue card %q appeared after a filler card", seed, card.ID)
			}
		}
	}
}

func TestOrderingPolicy_Apply_NonDefaultRatioInterleavesOneToOne(t *testing.T) {
	t.Parallel()

	// 3 new + 3 review. The default 4/5 ratio emits [N,N,R,N,R,R]; a 1/2 ratio
	// (new share 1, review share 1) alternates 1:1 starting with new, so new
	// cards land in the even slots and review cards in the odd slots. The
	// distinct slot composition proves the caller-supplied ratio reaches Apply.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("n1", domain.FSRSPhaseNew, base),
		dueCard("n2", domain.FSRSPhaseNew, base.Add(time.Minute)),
		dueCard("n3", domain.FSRSPhaseNew, base.Add(2*time.Minute)),
		dueCard("r1", domain.FSRSPhaseReview, base.Add(3*time.Minute)),
		dueCard("r2", domain.FSRSPhaseReview, base.Add(4*time.Minute)),
		dueCard("r3", domain.FSRSPhaseReview, base.Add(5*time.Minute)),
	}
	reviewSet := map[string]bool{"r1": true, "r2": true, "r3": true}

	ratio, err := domain.ParseNewCardRatio(1, 2)
	require.NoError(t, err)

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), ratio)
	require.Len(t, got, 6)

	for i, c := range got {
		if i%2 == 0 {
			require.False(t, reviewSet[c.ID], "slot %d must be a new card, got %q", i, c.ID)
		} else {
			require.True(t, reviewSet[c.ID], "slot %d must be a review card, got %q", i, c.ID)
		}
	}
}

// TestOrderingPolicy_Apply_EveryAcceptedRatioServesNewCardInSessionPrefix pins
// that every ratio ParseNewCardRatio accepts yields a new card within the first
// domain.DefaultLearnSessionSize slots.
func TestOrderingPolicy_Apply_EveryAcceptedRatioServesNewCardInSessionPrefix(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := make([]domain.DueCard, 0, 2*domain.DefaultLearnSessionSize)
	newSet := make(map[string]bool, domain.DefaultLearnSessionSize)
	for i := 0; i < domain.DefaultLearnSessionSize; i++ {
		id := fmt.Sprintf("new-%d", i)
		newSet[id] = true
		in = append(in, dueCard(id, domain.FSRSPhaseNew, base.Add(time.Duration(i)*time.Minute)))
	}
	for i := 0; i < domain.DefaultLearnSessionSize; i++ {
		in = append(in, dueCard(
			fmt.Sprintf("review-%d", i),
			domain.FSRSPhaseReview,
			base.Add(time.Duration(domain.DefaultLearnSessionSize+i)*time.Minute),
		))
	}

	for den := 2; den <= domain.NewCardRatioDenMax; den++ {
		for num := 1; num < den; num++ {
			ratio, err := domain.ParseNewCardRatio(num, den)
			if err != nil {
				continue
			}

			ordered := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)), ratio)
			hasNew := false
			for _, card := range ordered[:domain.DefaultLearnSessionSize] {
				if newSet[card.ID] {
					hasNew = true
					break
				}
			}
			require.True(t, hasNew,
				"accepted ratio %d/%d must serve a new card in the default session prefix", num, den)
		}
	}
}

func TestOrderingPolicy_Apply_SetEquality(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCard("n1", domain.FSRSPhaseNew, base),
		dueCard("n2", domain.FSRSPhaseNew, base.Add(time.Minute)),
		dueCard("r1", domain.FSRSPhaseReview, base),
		dueCard("r2", domain.FSRSPhaseReview, base),
		dueCard("r3", domain.FSRSPhaseLearning, base.Add(2*time.Minute)),
		dueCard("r4", domain.FSRSPhaseRelearning, base.Add(3*time.Minute)),
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
		{Card: nil, Phase: domain.FSRSPhaseNew},
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
	newC := []domain.DueCard{dueCard("new-0", domain.FSRSPhaseNew, base)}
	reviewC := make([]domain.DueCard, 0, 7)
	for i := 0; i < 7; i++ {
		reviewC = append(reviewC, dueCard(fmt.Sprintf("rev-%d", i), domain.FSRSPhaseReview, base.Add(time.Duration(i)*time.Minute)))
	}

	got := interleave(newC, reviewC, 4, 1)

	// Slot 1 goes to the new bucket (round(1*4/5) = 1) and empties it; the main
	// loop exits and the trailing-review path appends rev-0..rev-6.
	want := []string{"new-0", "rev-0", "rev-1", "rev-2", "rev-3", "rev-4", "rev-5", "rev-6"}
	require.Equal(t, want, cardIDs(got))
}

func TestInterleave_TrailingNewAppend(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	newC := make([]domain.DueCard, 0, 10)
	for i := 0; i < 10; i++ {
		newC = append(newC, dueCard(fmt.Sprintf("new-%d", i), domain.FSRSPhaseNew, base.Add(time.Duration(100+i)*time.Minute)))
	}
	reviewC := []domain.DueCard{
		dueCard("rev-0", domain.FSRSPhaseReview, base),
		dueCard("rev-1", domain.FSRSPhaseReview, base.Add(time.Minute)),
	}

	got := interleave(newC, reviewC, 4, 1)

	// round(k*4/5) hands slots 3 and 8 to the review bucket, emptying it; the
	// main loop exits and trailing-new appends new-6..new-9.
	want := []string{
		"new-0", "new-1", "rev-0", "new-2", "new-3",
		"new-4", "new-5", "rev-1", "new-6", "new-7",
		"new-8", "new-9",
	}
	require.Equal(t, want, cardIDs(got))
}

func TestInterleave_PanicsOnNonPositiveRatios(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	newC := []domain.DueCard{dueCard("new-0", domain.FSRSPhaseNew, base)}
	reviewC := []domain.DueCard{dueCard("rev-0", domain.FSRSPhaseReview, base)}

	require.Panics(t, func() { interleave(newC, reviewC, 0, 1) },
		"zero nRatio must panic")
	require.Panics(t, func() { interleave(newC, reviewC, 4, 0) },
		"zero rRatio must panic")
}

// deepBuckets builds one new and one review bucket of depth n plus the set of
// new-card ids, for interleave tests that must never hit a trailing-append path.
func deepBuckets(n int) (newC, reviewC []domain.DueCard, newSet map[string]bool) {
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	newSet = make(map[string]bool, n)
	newC = make([]domain.DueCard, 0, n)
	reviewC = make([]domain.DueCard, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("new-%d", i)
		newSet[id] = true
		newC = append(newC, dueCard(id, domain.FSRSPhaseNew, base.Add(time.Duration(i)*time.Minute)))
		reviewC = append(reviewC, dueCard(
			fmt.Sprintf("rev-%d", i),
			domain.FSRSPhaseReview,
			base.Add(time.Duration(n+i)*time.Minute),
		))
	}
	return newC, reviewC, newSet
}

// TestInterleave_PrefixFidelityAcrossAcceptedRatios pins the property the
// largest-remainder distribution buys: for every accepted ratio and every prefix
// length, the served new-card count stays within half a card of the nominal
// share. Both buckets stay deep so no trailing-append path is reached.
func TestInterleave_PrefixFidelityAcrossAcceptedRatios(t *testing.T) {
	t.Parallel()

	const bucketDepth = 120
	const maxPrefix = 100

	newC, reviewC, newSet := deepBuckets(bucketDepth)

	for den := 2; den <= domain.NewCardRatioDenMax; den++ {
		for num := 1; num < den; num++ {
			ratio, err := domain.ParseNewCardRatio(num, den)
			if err != nil {
				continue
			}

			ids := cardIDs(interleave(newC, reviewC, ratio.NewShare(), ratio.ReviewShare()))
			served := 0
			for k := 1; k <= maxPrefix; k++ {
				if newSet[ids[k-1]] {
					served++
				}
				nominal := float64(k) * float64(num) / float64(den)
				require.LessOrEqual(t, math.Abs(float64(served)-nominal), 0.5+1e-9,
					"ratio %d/%d prefix k=%d: served %d new, nominal %.4f", num, den, k, served, nominal)
			}
		}
	}
}

// TestInterleave_ServesNewCardEarlierThanTheOldCycle is the P1a regression: the
// removed cycle emission put den-num review cards ahead of the first new card,
// so the shipped 4/5 default served none in a one-card session. The first new
// card now lands at slot 1 for the default and at worst slot den-num elsewhere.
func TestInterleave_ServesNewCardEarlierThanTheOldCycle(t *testing.T) {
	t.Parallel()

	const bucketDepth = 120

	newC, reviewC, newSet := deepBuckets(bucketDepth)

	firstNewSlot := func(ratio domain.NewCardRatio) int {
		for i, id := range cardIDs(interleave(newC, reviewC, ratio.NewShare(), ratio.ReviewShare())) {
			if newSet[id] {
				return i + 1
			}
		}
		return -1
	}

	require.Equal(t, 1, firstNewSlot(domain.DefaultNewCardRatio),
		"the shipped 4/5 default must serve a new card in a one-card session")

	for den := 2; den <= domain.NewCardRatioDenMax; den++ {
		for num := 1; num < den; num++ {
			ratio, err := domain.ParseNewCardRatio(num, den)
			if err != nil {
				continue
			}
			require.LessOrEqual(t,
				firstNewSlot(ratio), ratio.Denominator()-ratio.Numerator(),
				"ratio %d/%d must serve its first new card no later than slot den-num", num, den)
		}
	}
}

// TestInterleave_AdvertisedDefaultSessionSplit pins the advertised composition:
// a full-pool default session is exactly 16 new / 4 review, the split the
// removed cycle emission also produced at whole multiples of the denominator.
func TestInterleave_AdvertisedDefaultSessionSplit(t *testing.T) {
	t.Parallel()

	newC, reviewC, newSet := deepBuckets(2 * domain.DefaultLearnSessionSize)
	ratio := domain.DefaultNewCardRatio

	ids := cardIDs(interleave(newC, reviewC, ratio.NewShare(), ratio.ReviewShare()))
	served := 0
	for _, id := range ids[:domain.DefaultLearnSessionSize] {
		if newSet[id] {
			served++
		}
	}

	require.Equal(t, 16, served, "a default 20-card session must serve 16 new cards")
	require.Equal(t, 4, domain.DefaultLearnSessionSize-served, "and 4 review cards")
}
