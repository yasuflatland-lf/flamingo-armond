package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/repository"
)

const (
	// strugglingCardsLimit caps the struggling-card list returned to the client.
	strugglingCardsLimit = 10
	// statsWindowDays is the trailing calendar window (in days) over which the
	// diagnostic performance snapshot is computed, so studyStreak reflects real
	// calendar days rather than a swipe-count window.
	statsWindowDays = 365
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

// statsSwipeRepo is the narrow slice of the swipe-record repository the stats
// aggregate needs to compute the diagnostic performance snapshot over a window.
type statsSwipeRepo interface {
	ListByUserSince(ctx context.Context, userID string, since time.Time) ([]*domain.SwipeRecord, error)
}

type statsUsecase struct {
	fsrsRepo  statsFSRSRepo
	swipeRepo statsSwipeRepo
	clock     Clock
}

// NewStats wires the stats usecase from its narrow FSRS + swipe-record
// repository dependencies and an injectable Clock (nil defaults to
// systemClock{} in production; tests inject a fixed Clock so the window/now
// are deterministic). Clock is the same interface NewLearnUsecase uses.
func NewStats(fsrsRepo statsFSRSRepo, swipeRepo statsSwipeRepo, clock Clock) StatsUsecase {
	if clock == nil {
		clock = systemClock{}
	}
	return &statsUsecase{fsrsRepo: fsrsRepo, swipeRepo: swipeRepo, clock: clock}
}

// LearningStatsResult is the usecase-layer result VO. It carries cardgroup ids
// and struggling-card ids only; the resolver hydrates the Cardgroup / Card
// objects via the DataLoader.
type LearningStatsResult struct {
	Mastery         MasteryBreakdown
	Decks           []DeckMasteryResult
	Performance     service.PerformanceMetrics
	StrugglingCards []StrugglingCardResult
}

// StrugglingCardResult identifies a card the learner struggles with (high
// lapses / low stability). CardID is hydrated to a Card by the resolver.
type StrugglingCardResult struct {
	CardID    string
	Lapses    int
	Stability float64
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
		default:
			return nil, eris.Errorf("usecase: stats: unhandled MasteryTier %d", tier)
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

	// Diagnostic half: a performance snapshot over the trailing statsWindowDays
	// calendar window (so studyStreak reflects real days, not a swipe count) plus
	// the struggling-card ranking derived from the FSRS rows already loaded above.
	now := u.clock.Now()
	swipes, err := u.swipeRepo.ListByUserSince(ctx, caller.Sub, now.AddDate(0, 0, -statsWindowDays))
	if err != nil {
		return nil, eris.Wrap(err, "usecase: stats: list swipes since")
	}

	return &LearningStatsResult{
		Mastery:         mastery,
		Decks:           decks,
		Performance:     service.ComputeMetrics(swipeRecordsByValue(swipes), now),
		StrugglingCards: topStruggling(states, strugglingCardsLimit),
	}, nil
}

// topStruggling ranks the studied FSRS rows by struggle: filter to cards with at
// least one lapse, sort by (Lapses desc, Stability asc), and cap at limit. The
// result is always non-nil (an empty, non-nil slice when no card has lapsed).
func topStruggling(states []repository.FSRSStatRow, limit int) []StrugglingCardResult {
	out := make([]StrugglingCardResult, 0, len(states))
	for _, s := range states {
		if s.Lapses >= 1 {
			out = append(out, StrugglingCardResult{CardID: s.CardID, Lapses: s.Lapses, Stability: s.Stability})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Lapses != out[j].Lapses {
			return out[i].Lapses > out[j].Lapses // more lapses first
		}
		return out[i].Stability < out[j].Stability // ties: lower stability first
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}
