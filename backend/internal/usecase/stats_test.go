package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/usecase/ucerr"
)

type fakeStatsFSRSRepo struct {
	states    []domain.FSRSStat
	totals    map[string]int
	statesErr error
	totalsErr error
}

func (f *fakeStatsFSRSRepo) ListFSRSStatesByUser(_ context.Context, _ string) ([]domain.FSRSStat, error) {
	if f.statesErr != nil {
		return nil, f.statesErr
	}
	return f.states, nil
}

func (f *fakeStatsFSRSRepo) CountCardsByCardgroupForUser(_ context.Context, _ string) (map[string]int, error) {
	if f.totalsErr != nil {
		return nil, f.totalsErr
	}
	return f.totals, nil
}

// fakeStatsSwipeRepo captures the ListByUserSince arguments so tests can pin the
// windowing (since) and tenant scoping, and returns a canned swipe slice.
type fakeStatsSwipeRepo struct {
	swipes    []*domain.SwipeRecord
	err       error
	called    bool
	userIDArg string
	sinceArg  time.Time
}

func (f *fakeStatsSwipeRepo) ListByUserSince(_ context.Context, userID string, since time.Time) ([]*domain.SwipeRecord, error) {
	f.called = true
	f.userIDArg = userID
	f.sinceArg = since
	if f.err != nil {
		return nil, f.err
	}
	return f.swipes, nil
}

// fakeStatsCardgroupRepo returns a configurable owned-deck count (and optional
// error) for CountByOwner so tests can pin the OwnsAnyDeck signal without a DB.
type fakeStatsCardgroupRepo struct {
	count int64
	err   error
}

func (f *fakeStatsCardgroupRepo) CountByOwner(_ context.Context, _ string, _ *string) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.count, nil
}

func authedStatsCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
}

func strugglingRow(card string, lapses int, stability float64) domain.FSRSStat {
	return domain.FSRSStat{CardID: card, CardgroupID: "cg1", Phase: domain.FSRSPhaseReview, Stability: stability, Lapses: lapses}
}

func strugglingCardIDs(cards []service.StrugglingCard) []string {
	out := make([]string, 0, len(cards))
	for _, c := range cards {
		out = append(out, c.CardID)
	}
	return out
}

func reviewRow(card, cg string, stability float64) domain.FSRSStat {
	return domain.FSRSStat{CardID: card, CardgroupID: cg, Phase: domain.FSRSPhaseReview, Stability: stability}
}

func phaseRow(card, cg string, phase domain.FSRSPhase) domain.FSRSStat {
	return domain.FSRSStat{CardID: card, CardgroupID: cg, Phase: phase}
}

func TestStatsUsecase_MyLearningStats_BucketsGlobalAndPerDeck(t *testing.T) {
	t.Parallel()

	repo := &fakeStatsFSRSRepo{
		states: []domain.FSRSStat{
			reviewRow("a", "cg1", 30),                         // mature
			reviewRow("b", "cg1", 10),                         // learned
			phaseRow("c", "cg1", domain.FSRSPhaseNew),         // in progress
			reviewRow("d", "cg2", domain.MatureStabilityDays), // mature (boundary, >=)
			phaseRow("e", "cg2", domain.FSRSPhaseLearning),    // in progress
		},
		totals: map[string]int{"cg1": 5, "cg2": 3, "cg3": 2},
	}
	uc := NewStats(repo, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	require.NotNil(t, res)

	// Global breakdown; the three tiers sum to TotalStudied = len(states).
	assert.Equal(t, 2, res.Mastery.InProgress)
	assert.Equal(t, 1, res.Mastery.Learned)
	assert.Equal(t, 2, res.Mastery.Mature)
	assert.Equal(t, 5, res.Mastery.TotalStudied)
	assert.Equal(t, len(repo.states), res.Mastery.TotalStudied)
	assert.Equal(t,
		res.Mastery.TotalStudied,
		res.Mastery.InProgress+res.Mastery.Learned+res.Mastery.Mature,
		"tiers are disjoint and cover every studied card")

	// A DeckMastery is emitted for every deck in totals, in deterministic
	// (CardgroupID-sorted) order — including cg3 with zero studied cards.
	byDeck := make(map[string]service.DeckMastery, len(res.Decks))
	order := make([]string, 0, len(res.Decks))
	for _, d := range res.Decks {
		byDeck[d.CardgroupID] = d
		order = append(order, d.CardgroupID)
	}
	require.Len(t, res.Decks, 3)
	assert.Equal(t, []string{"cg1", "cg2", "cg3"}, order)

	assert.Equal(t, service.DeckMastery{CardgroupID: "cg1", TotalCards: 5, LearnedCards: 1, MatureCards: 1}, byDeck["cg1"])
	assert.Equal(t, service.DeckMastery{CardgroupID: "cg2", TotalCards: 3, LearnedCards: 0, MatureCards: 1}, byDeck["cg2"])
	assert.Equal(t, service.DeckMastery{CardgroupID: "cg3", TotalCards: 2, LearnedCards: 0, MatureCards: 0}, byDeck["cg3"],
		"a deck with zero studied cards is still emitted with learned=mature=0")
	for _, d := range res.Decks {
		assert.LessOrEqual(t, d.LearnedCards+d.MatureCards, d.TotalCards,
			"acquired never exceeds the denominator for deck %s", d.CardgroupID)
	}
}

func TestStatsUsecase_MyLearningStats_EmptyHistory(t *testing.T) {
	t.Parallel()
	repo := &fakeStatsFSRSRepo{states: nil, totals: map[string]int{}}
	uc := NewStats(repo, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	assert.Equal(t, service.MasteryBreakdown{}, res.Mastery)
	assert.Empty(t, res.Decks)
}

func TestStatsUsecase_MyLearningStats_Unauthenticated(t *testing.T) {
	t.Parallel()
	uc := NewStats(&fakeStatsFSRSRepo{}, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{}, nil)

	res, err := uc.MyLearningStats(context.Background())
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, ucerr.ErrUnauthenticated),
		"unauthenticated caller returns the bare sentinel, not a wrapped error")
}

func TestStatsUsecase_MyLearningStats_EmptySubUnauthenticated(t *testing.T) {
	t.Parallel()
	uc := NewStats(&fakeStatsFSRSRepo{}, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx(""))
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, ucerr.ErrUnauthenticated))
}

func TestStatsUsecase_MyLearningStats_ListStatesError(t *testing.T) {
	t.Parallel()
	sentinel := eris.New("boom")
	uc := NewStats(&fakeStatsFSRSRepo{statesErr: sentinel}, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, sentinel))
	assert.Contains(t, err.Error(), "usecase: stats: list fsrs states")
}

func TestStatsUsecase_MyLearningStats_CountError(t *testing.T) {
	t.Parallel()
	sentinel := eris.New("boom")
	uc := NewStats(&fakeStatsFSRSRepo{totalsErr: sentinel}, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, sentinel))
	assert.Contains(t, err.Error(), "usecase: stats: count cards by cardgroup")
}

func TestStatsUsecase_MyLearningStats_WindowsSwipesByStatsWindowDays(t *testing.T) {
	t.Parallel()
	fixedNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	swipeRepo := &fakeStatsSwipeRepo{}
	uc := NewStats(&fakeStatsFSRSRepo{totals: map[string]int{}}, swipeRepo, &fakeStatsCardgroupRepo{}, fixedClock{now: fixedNow})

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, swipeRepo.called, "MyLearningStats reads the windowed swipe history")
	assert.Equal(t, "user-1", swipeRepo.userIDArg, "scoped to the caller")
	assert.Equal(t, fixedNow.AddDate(0, 0, -365), swipeRepo.sinceArg,
		"since == now minus statsWindowDays (365) using the injected clock")
	assert.NotNil(t, res.StrugglingCards, "struggling cards is always a non-nil slice")
	assert.Empty(t, res.StrugglingCards)
}

func TestStatsUsecase_MyLearningStats_WindowsReflectComputeWindowedMetrics(t *testing.T) {
	t.Parallel()
	fixedNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	state := domain.NewFSRSStateForNewCard(fixedNow)
	state.Phase = domain.FSRSPhaseReview
	swipes := []*domain.SwipeRecord{
		{ID: "s1", UserID: "user-1", CardID: "c1", CardgroupID: "cg1", Rating: domain.RatingEasy, ReviewedAt: fixedNow, StateAfter: state},
		{ID: "s2", UserID: "user-1", CardID: "c2", CardgroupID: "cg1", Rating: domain.RatingAgain, ReviewedAt: fixedNow.AddDate(0, 0, -1), StateAfter: state},
	}
	swipeRepo := &fakeStatsSwipeRepo{swipes: swipes}
	uc := NewStats(&fakeStatsFSRSRepo{totals: map[string]int{}}, swipeRepo, &fakeStatsCardgroupRepo{}, fixedClock{now: fixedNow})

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	require.NotNil(t, res)

	want := service.ComputeWindowedMetrics(swipeRecordsByValue(swipes), fixedNow)
	assert.Equal(t, want, res.Windows,
		"Windows is ComputeWindowedMetrics over the loaded swipes at the injected now")
}

func TestStatsUsecase_MyLearningStats_StrugglingCardsFromFSRSRows(t *testing.T) {
	t.Parallel()
	fixedNow := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	repo := &fakeStatsFSRSRepo{
		states: []domain.FSRSStat{
			strugglingRow("clean", 0, 20), // excluded (no lapses)
			strugglingRow("worst", 4, 2),  // most lapses
			strugglingRow("mid", 2, 15),
			strugglingRow("mid-fragile", 2, 3), // ties on lapses with "mid"; lower stability ranks first
		},
		totals: map[string]int{},
	}
	uc := NewStats(repo, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{}, fixedClock{now: fixedNow})

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, []string{"worst", "mid-fragile", "mid"}, strugglingCardIDs(res.StrugglingCards))
	assert.Equal(t, service.StrugglingCard{CardID: "worst", Lapses: 4, Stability: 2}, res.StrugglingCards[0])
}

func TestStatsUsecase_MyLearningStats_ListSwipesError(t *testing.T) {
	t.Parallel()
	sentinel := eris.New("boom")
	uc := NewStats(&fakeStatsFSRSRepo{totals: map[string]int{}}, &fakeStatsSwipeRepo{err: sentinel}, &fakeStatsCardgroupRepo{}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, sentinel))
	assert.Contains(t, err.Error(), "usecase: stats: list swipes since")
}

// TestStatsUsecase_MyLearningStats_OwnsAnyDeck_TrulyNew covers a brand-new user
// who owns no cardgroup: CountByOwner returns 0, so OwnsAnyDeck is false and
// Decks is empty.
func TestStatsUsecase_MyLearningStats_OwnsAnyDeck_TrulyNew(t *testing.T) {
	t.Parallel()
	repo := &fakeStatsFSRSRepo{states: nil, totals: map[string]int{}}
	uc := NewStats(repo, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{count: 0}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.False(t, res.OwnsAnyDeck, "a user owning no cardgroup is not OwnsAnyDeck")
	assert.Empty(t, res.Decks)
}

// TestStatsUsecase_MyLearningStats_OwnsAnyDeck_EmptyDeck covers a user who owns
// a cardgroup with zero cards and has studied nothing: CountByOwner returns 1,
// so OwnsAnyDeck is true even though Decks (built from card totals) is empty.
func TestStatsUsecase_MyLearningStats_OwnsAnyDeck_EmptyDeck(t *testing.T) {
	t.Parallel()
	repo := &fakeStatsFSRSRepo{states: nil, totals: map[string]int{}}
	uc := NewStats(repo, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{count: 1}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.OwnsAnyDeck, "owning an empty deck still counts as OwnsAnyDeck")
	assert.Empty(t, res.Decks, "an empty deck is omitted from Decks (no card totals)")
}

// TestStatsUsecase_MyLearningStats_OwnsAnyDeck_DeckWithCards covers a user who
// owns a non-empty deck and has studied cards: CountByOwner returns 1, Decks is
// non-empty, and OwnsAnyDeck is true.
func TestStatsUsecase_MyLearningStats_OwnsAnyDeck_DeckWithCards(t *testing.T) {
	t.Parallel()
	repo := &fakeStatsFSRSRepo{
		states: []domain.FSRSStat{reviewRow("a", "cg1", 30)},
		totals: map[string]int{"cg1": 1},
	}
	uc := NewStats(repo, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{count: 1}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.True(t, res.OwnsAnyDeck)
	require.Len(t, res.Decks, 1)
	assert.Equal(t, "cg1", res.Decks[0].CardgroupID)
}

// TestStatsUsecase_MyLearningStats_CountByOwnerError covers a CountByOwner
// failure: MyLearningStats returns the eris-wrapped error.
func TestStatsUsecase_MyLearningStats_CountByOwnerError(t *testing.T) {
	t.Parallel()
	sentinel := eris.New("boom")
	uc := NewStats(&fakeStatsFSRSRepo{totals: map[string]int{}}, &fakeStatsSwipeRepo{}, &fakeStatsCardgroupRepo{err: sentinel}, nil)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, sentinel))
	assert.Contains(t, err.Error(), "usecase: stats: count cardgroups by owner")
}
