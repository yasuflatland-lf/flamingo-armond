package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

type fakeStatsFSRSRepo struct {
	states    []repository.FSRSStatRow
	totals    map[string]int
	statesErr error
	totalsErr error
}

func (f *fakeStatsFSRSRepo) ListFSRSStatesByUser(_ context.Context, _ string) ([]repository.FSRSStatRow, error) {
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

func authedStatsCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
}

func reviewRow(card, cg string, stability float64) repository.FSRSStatRow {
	return repository.FSRSStatRow{CardID: card, CardgroupID: cg, Phase: domain.FSRSPhaseReview, Stability: stability}
}

func phaseRow(card, cg string, phase domain.FSRSPhase) repository.FSRSStatRow {
	return repository.FSRSStatRow{CardID: card, CardgroupID: cg, Phase: phase}
}

func TestStatsUsecase_MyLearningStats_BucketsGlobalAndPerDeck(t *testing.T) {
	t.Parallel()

	repo := &fakeStatsFSRSRepo{
		states: []repository.FSRSStatRow{
			reviewRow("a", "cg1", 30),                         // mature
			reviewRow("b", "cg1", 10),                         // learned
			phaseRow("c", "cg1", domain.FSRSPhaseNew),         // in progress
			reviewRow("d", "cg2", domain.MatureStabilityDays), // mature (boundary, >=)
			phaseRow("e", "cg2", domain.FSRSPhaseLearning),    // in progress
		},
		totals: map[string]int{"cg1": 5, "cg2": 3, "cg3": 2},
	}
	uc := NewStats(repo)

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

	// A DeckMasteryResult is emitted for every deck in totals, in deterministic
	// (CardgroupID-sorted) order — including cg3 with zero studied cards.
	byDeck := make(map[string]DeckMasteryResult, len(res.Decks))
	order := make([]string, 0, len(res.Decks))
	for _, d := range res.Decks {
		byDeck[d.CardgroupID] = d
		order = append(order, d.CardgroupID)
	}
	require.Len(t, res.Decks, 3)
	assert.Equal(t, []string{"cg1", "cg2", "cg3"}, order)

	assert.Equal(t, DeckMasteryResult{CardgroupID: "cg1", TotalCards: 5, LearnedCards: 1, MatureCards: 1}, byDeck["cg1"])
	assert.Equal(t, DeckMasteryResult{CardgroupID: "cg2", TotalCards: 3, LearnedCards: 0, MatureCards: 1}, byDeck["cg2"])
	assert.Equal(t, DeckMasteryResult{CardgroupID: "cg3", TotalCards: 2, LearnedCards: 0, MatureCards: 0}, byDeck["cg3"],
		"a deck with zero studied cards is still emitted with learned=mature=0")
	for _, d := range res.Decks {
		assert.LessOrEqual(t, d.LearnedCards+d.MatureCards, d.TotalCards,
			"acquired never exceeds the denominator for deck %s", d.CardgroupID)
	}
}

func TestStatsUsecase_MyLearningStats_EmptyHistory(t *testing.T) {
	t.Parallel()
	repo := &fakeStatsFSRSRepo{states: nil, totals: map[string]int{}}
	uc := NewStats(repo)

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.NoError(t, err)
	assert.Equal(t, MasteryBreakdown{}, res.Mastery)
	assert.Empty(t, res.Decks)
}

func TestStatsUsecase_MyLearningStats_Unauthenticated(t *testing.T) {
	t.Parallel()
	uc := NewStats(&fakeStatsFSRSRepo{})

	res, err := uc.MyLearningStats(context.Background())
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, ucerr.ErrUnauthenticated),
		"unauthenticated caller returns the bare sentinel, not a wrapped error")
}

func TestStatsUsecase_MyLearningStats_EmptySubUnauthenticated(t *testing.T) {
	t.Parallel()
	uc := NewStats(&fakeStatsFSRSRepo{})

	res, err := uc.MyLearningStats(authedStatsCtx(""))
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, ucerr.ErrUnauthenticated))
}

func TestStatsUsecase_MyLearningStats_ListStatesError(t *testing.T) {
	t.Parallel()
	sentinel := eris.New("boom")
	uc := NewStats(&fakeStatsFSRSRepo{statesErr: sentinel})

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, sentinel))
	assert.Contains(t, err.Error(), "usecase: stats: list fsrs states")
}

func TestStatsUsecase_MyLearningStats_CountError(t *testing.T) {
	t.Parallel()
	sentinel := eris.New("boom")
	uc := NewStats(&fakeStatsFSRSRepo{totalsErr: sentinel})

	res, err := uc.MyLearningStats(authedStatsCtx("user-1"))
	require.Error(t, err)
	assert.Nil(t, res)
	assert.True(t, errors.Is(err, sentinel))
	assert.Contains(t, err.Error(), "usecase: stats: count cards by cardgroup")
}
