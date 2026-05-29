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

// dueCardPos builds a DueCard fixture with an explicit Card.Position. Use this
// for new-card fixtures where a deterministic post-shuffle order is asserted:
// distinct positions collapse to size-1 runs, so shuffleSamePosition is a no-op.
func dueCardPos(id string, state domain.FSRSCardState, due time.Time, pos int) domain.DueCard {
	return domain.DueCard{
		Card:  &domain.Card{ID: id, Position: pos},
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

	// Distinct positions (0,1,2) so shuffleSamePosition is a no-op and the
	// post-partition order is identical to the input order.
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCardPos("n1", domain.FSRSStateNew, base, 0),
		dueCardPos("n2", domain.FSRSStateNew, base.Add(time.Minute), 1),
		dueCardPos("n3", domain.FSRSStateNew, base.Add(2*time.Minute), 2),
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

	// 5 new (distinct positions 0..4) + 20 review (distinct Due timestamps)
	// so both shuffle steps are no-ops and we can assert the deterministic
	// interleave order. Expected pattern: review-first, ReviewCardRatio (4)
	// reviews then NewCardRatio (1) new, repeating until both buckets empty.
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
		in = append(in, dueCardPos(
			fmt.Sprintf("new-%d", i),
			domain.FSRSStateNew,
			base.Add(time.Duration(100+i)*time.Minute),
			i, // distinct positions 0..4 => shuffleSamePosition is a no-op
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
	// shuffle to [c, d, a, b].
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

// TestOrderingPolicy_Apply_TrailingReviewAppend exercises the trailing-review
// append path in interleave. Fixture: 1 new (Position 0, single-card run →
// shuffleSamePosition no-op) + 7 review (distinct Due timestamps →
// shuffleSameDue no-op).
//
// With ReviewCardRatio=4 and NewCardRatio=1 the outer loop runs one full cycle:
// emit rev-0..rev-3 then new-0. After that cycle i=1 == len(newC)=1, so the
// outer loop exits. The trailing-review append path ("for ; j < len(reviewC)")
// then appends rev-4, rev-5, rev-6.
func TestOrderingPolicy_Apply_TrailingReviewAppend(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := make([]domain.DueCard, 0, 8)
	for i := 0; i < 7; i++ {
		in = append(in, dueCard(
			fmt.Sprintf("rev-%d", i),
			domain.FSRSStateReview,
			base.Add(time.Duration(i)*time.Minute),
		))
	}
	in = append(in, dueCard(
		"new-0",
		domain.FSRSStateNew,
		base.Add(100*time.Minute),
	))

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))

	// One full cycle emits rev-0..rev-3 + new-0; new bucket exhausts and the
	// trailing-review append path emits rev-4, rev-5, rev-6.
	want := []string{
		"rev-0", "rev-1", "rev-2", "rev-3", "new-0",
		"rev-4", "rev-5", "rev-6",
	}
	require.Len(t, got, 8)
	require.Equal(t, want, cardIDs(got))
}

// TestOrderingPolicy_Apply_TrailingNewAppend exercises the trailing-new append
// path in interleave. Fixture: 5 new (distinct positions 0..4) + 3 review
// (distinct Due timestamps) so both shuffle steps are no-ops.
//
// With ReviewCardRatio=4 and NewCardRatio=1 the k-loop guard "j < len(reviewC)"
// exits after emitting rev-0, rev-1, rev-2 (only 3 reviews exist), then new-0
// is emitted. After that j=3 == len(reviewC)=3, so the outer loop exits. The
// trailing-new append path ("for ; i < len(newC)") then appends new-1..new-4.
func TestOrderingPolicy_Apply_TrailingNewAppend(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := make([]domain.DueCard, 0, 8)
	for i := 0; i < 3; i++ {
		in = append(in, dueCard(
			fmt.Sprintf("rev-%d", i),
			domain.FSRSStateReview,
			base.Add(time.Duration(i)*time.Minute),
		))
	}
	for i := 0; i < 5; i++ {
		in = append(in, dueCardPos(
			fmt.Sprintf("new-%d", i),
			domain.FSRSStateNew,
			base.Add(time.Duration(100+i)*time.Minute),
			i, // distinct positions 0..4 => shuffleSamePosition is a no-op
		))
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))

	// The k-loop emits rev-0, rev-1, rev-2 then exits early (review bucket
	// exhausted inside the k-loop guard); new-0 is emitted next. The review
	// bucket is now empty so the outer loop exits. The trailing-new append
	// path emits new-1, new-2, new-3, new-4.
	want := []string{
		"rev-0", "rev-1", "rev-2", "new-0",
		"new-1", "new-2", "new-3", "new-4",
	}
	require.Len(t, got, 8)
	require.Equal(t, want, cardIDs(got))
}

// TestOrderingPolicy_Apply_MultipleSameDueRuns verifies that two disjoint
// same-Due runs are each shuffled independently and that run-1 (earlier Due)
// is emitted entirely before run-2 (later Due).
//
// Fixture: run-1 has two review cards at T1; run-2 has two review cards at T2
// where T1 < T2. With rand.NewSource(42) the shuffle produces a deterministic
// permutation within each run; the inter-run order is unchanged (T1 before T2).
func TestOrderingPolicy_Apply_MultipleSameDueRuns(t *testing.T) {
	t.Parallel()

	T1 := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	T2 := T1.Add(time.Hour)

	in := []domain.DueCard{
		dueCard("r1a", domain.FSRSStateReview, T1),
		dueCard("r1b", domain.FSRSStateReview, T1),
		dueCard("r2a", domain.FSRSStateReview, T2),
		dueCard("r2b", domain.FSRSStateReview, T2),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))
	ids := cardIDs(got)

	require.Len(t, ids, 4)

	// Both run-1 cards appear before both run-2 cards (inter-run order preserved).
	run1Set := map[string]bool{"r1a": true, "r1b": true}
	run2Set := map[string]bool{"r2a": true, "r2b": true}
	lastRun1Idx := -1
	firstRun2Idx := len(ids)
	for i, id := range ids {
		if run1Set[id] {
			lastRun1Idx = i
		}
		if run2Set[id] && i < firstRun2Idx {
			firstRun2Idx = i
		}
	}
	require.Less(t, lastRun1Idx, firstRun2Idx,
		"all run-1 cards must appear before all run-2 cards")

	// With rand.NewSource(42) each 2-card run is shuffled deterministically.
	// Probed result: run-1 permutes to [r1b, r1a]; run-2 permutes to [r2b, r2a].
	require.Equal(t, []string{"r1b", "r1a", "r2b", "r2a"}, ids,
		"deterministic shuffle order for seed 42")
}

// TestOrderingPolicy_Apply_NewCards_DistinctPositions_Deterministic verifies
// that new cards with distinct positions are never reshuffled: distinct
// positions collapse to size-1 runs, so shuffleSamePosition is a no-op and the
// output equals position order regardless of the rng seed.
func TestOrderingPolicy_Apply_NewCards_DistinctPositions_Deterministic(t *testing.T) {
	t.Parallel()

	// All cards share the same Due (the FSRS tie); the repository delivers them
	// pre-sorted by position ASC. Distinct positions 0,1,2,3 => no shuffle.
	due := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCardPos("p0", domain.FSRSStateNew, due, 0),
		dueCardPos("p1", domain.FSRSStateNew, due, 1),
		dueCardPos("p2", domain.FSRSStateNew, due, 2),
		dueCardPos("p3", domain.FSRSStateNew, due, 3),
	}
	want := []string{"p0", "p1", "p2", "p3"}

	got1 := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))
	require.Equal(t, want, cardIDs(got1),
		"distinct positions must stay in position order")

	// A different seed must produce the identical order: distinct positions are
	// never reshuffled.
	got2 := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(7)))
	require.Equal(t, want, cardIDs(got2),
		"distinct positions must be seed-independent (never reshuffled)")
}

// TestOrderingPolicy_Apply_NewCards_MultipleEqualPositionRuns verifies that
// shuffleSamePosition correctly handles multiple disjoint equal-Position runs.
// Fixture: pos0-a and pos0-b share Position 0; pos1-c and pos1-d share Position 1.
// The inter-run order (pos-0 run before pos-1 run) must be preserved; within
// each 2-card run the order is the deterministic seed-42 permutation.
func TestOrderingPolicy_Apply_NewCards_MultipleEqualPositionRuns(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCardPos("pos0-a", domain.FSRSStateNew, due, 0),
		dueCardPos("pos0-b", domain.FSRSStateNew, due, 0),
		dueCardPos("pos1-c", domain.FSRSStateNew, due, 1),
		dueCardPos("pos1-d", domain.FSRSStateNew, due, 1),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))
	gotIDs := cardIDs(got)

	require.Len(t, gotIDs, 4)

	// Inter-run order preserved: all pos-0 cards appear before all pos-1 cards.
	pos0Set := map[string]bool{"pos0-a": true, "pos0-b": true}
	pos1Set := map[string]bool{"pos1-c": true, "pos1-d": true}
	lastPos0Idx := -1
	firstPos1Idx := len(gotIDs)
	for i, id := range gotIDs {
		if pos0Set[id] {
			lastPos0Idx = i
		}
		if pos1Set[id] && i < firstPos1Idx {
			firstPos1Idx = i
		}
	}
	require.Less(t, lastPos0Idx, firstPos1Idx,
		"all pos-0 cards must appear before all pos-1 cards")

	// Pinned deterministic order for rand.NewSource(42):
	// pos-0 run [pos0-a, pos0-b] shuffles to [pos0-b, pos0-a];
	// pos-1 run [pos1-c, pos1-d] shuffles to [pos1-d, pos1-c].
	require.Equal(t, []string{"pos0-b", "pos0-a", "pos1-d", "pos1-c"}, gotIDs,
		"deterministic shuffle order for seed 42")
}

// TestOrderingPolicy_Apply_NewCards_EqualPositions_Shuffled verifies that new
// cards all sharing Position 0 (the non-Notion case) form a single equal-Position
// run and are shuffled, preserving the "don't show the same first-N" property.
func TestOrderingPolicy_Apply_NewCards_EqualPositions_Shuffled(t *testing.T) {
	t.Parallel()

	// Every card has Position 0 => one equal-Position run => shuffled.
	due := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	in := []domain.DueCard{
		dueCardPos("a", domain.FSRSStateNew, due, 0),
		dueCardPos("b", domain.FSRSStateNew, due, 0),
		dueCardPos("c", domain.FSRSStateNew, due, 0),
		dueCardPos("d", domain.FSRSStateNew, due, 0),
	}

	got := NewOrderingPolicy().Apply(in, rand.New(rand.NewSource(42)))
	gotIDs := cardIDs(got)

	require.Len(t, gotIDs, 4)
	require.NotEqual(t, []string{"a", "b", "c", "d"}, gotIDs,
		"equal-Position run must be shuffled, not left in input order")
	// With rand.NewSource(42), four identical-Position cards [a, b, c, d]
	// shuffle to [c, d, a, b] (same permutation the same-Due shuffle produces).
	require.Equal(t, []string{"c", "d", "a", "b"}, gotIDs,
		"deterministic shuffle order for seed 42")
}
