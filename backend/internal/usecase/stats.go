package usecase

import (
	"context"
	"sort"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

// StatsUsecase exposes the authenticated caller's aggregate learning
// statistics.
type StatsUsecase interface {
	MyLearningStats(ctx context.Context) (*LearningStatsResult, error)
}

// statsFSRSRepo is the narrow, consumer-defined slice of the FSRS repository the
// stats aggregate needs. Declaring it here keeps the dependency arrow correct
// and lets a fake satisfy it in tests.
type statsFSRSRepo interface {
	ListFSRSStatesByUser(ctx context.Context, userID string) ([]repository.FSRSStatRow, error)
	CountCardsByCardgroupForUser(ctx context.Context, userID string) (map[string]int, error)
}

type statsUsecase struct {
	fsrsRepo statsFSRSRepo
}

// NewStats wires the stats usecase from its narrow FSRS repository dependency.
func NewStats(fsrsRepo statsFSRSRepo) StatsUsecase { return &statsUsecase{fsrsRepo: fsrsRepo} }

// LearningStatsResult is the usecase-layer result VO. It carries cardgroup ids
// only; the resolver hydrates the Cardgroup objects via the DataLoader.
type LearningStatsResult struct {
	Mastery MasteryBreakdown
	Decks   []DeckMasteryResult
}

// MasteryBreakdown is the disjoint three-tier count across all studied cards.
// InProgress + Learned + Mature == TotalStudied by construction.
type MasteryBreakdown struct{ InProgress, Learned, Mature, TotalStudied int }

// DeckMasteryResult is a per-deck acquisition summary. LearnedCards and
// MatureCards are disjoint; acquired == LearnedCards + MatureCards.
type DeckMasteryResult struct {
	CardgroupID  string
	TotalCards   int
	LearnedCards int // disjoint: Review & stability < MatureStabilityDays
	MatureCards  int // disjoint: Review & stability >= MatureStabilityDays
}

func (u *statsUsecase) MyLearningStats(ctx context.Context) (*LearningStatsResult, error) {
	caller := auth.UserFrom(ctx)
	if err := requireCallerSub(caller); err != nil {
		return nil, err // ucerr.ErrUnauthenticated — must stay unwrapped.
	}

	states, err := u.fsrsRepo.ListFSRSStatesByUser(ctx, caller.Sub)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: stats: list fsrs states")
	}
	totals, err := u.fsrsRepo.CountCardsByCardgroupForUser(ctx, caller.Sub)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: stats: count cards by cardgroup")
	}

	// perDeck accumulates the disjoint learned/mature split per cardgroup while
	// the global breakdown accrues across every studied card.
	type deckAcc struct{ learned, mature int }
	perDeck := make(map[string]*deckAcc, len(totals))
	mastery := MasteryBreakdown{TotalStudied: len(states)}
	for _, s := range states {
		tier := domain.ClassifyMastery(
			domain.FSRSState{Phase: s.Phase, Stability: s.Stability},
			domain.MatureStabilityDays,
		)
		acc := perDeck[s.CardgroupID]
		if acc == nil {
			acc = &deckAcc{}
			perDeck[s.CardgroupID] = acc
		}
		switch tier {
		case domain.TierInProgress:
			mastery.InProgress++
		case domain.TierLearned:
			mastery.Learned++
			acc.learned++
		case domain.TierMature:
			mastery.Mature++
			acc.mature++
		}
	}

	// Emit one DeckMasteryResult for every owned deck (from totals), so a deck
	// with zero studied cards still appears with learned=mature=0. Sort by
	// CardgroupID for a deterministic wire order.
	deckIDs := make([]string, 0, len(totals))
	for id := range totals {
		deckIDs = append(deckIDs, id)
	}
	sort.Strings(deckIDs)

	decks := make([]DeckMasteryResult, 0, len(deckIDs))
	for _, id := range deckIDs {
		acc := perDeck[id]
		var learned, mature int
		if acc != nil {
			learned, mature = acc.learned, acc.mature
		}
		decks = append(decks, DeckMasteryResult{
			CardgroupID:  id,
			TotalCards:   totals[id],
			LearnedCards: learned,
			MatureCards:  mature,
		})
	}

	return &LearningStatsResult{Mastery: mastery, Decks: decks}, nil
}
