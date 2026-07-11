package usecase

import (
	"context"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
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
	ListFSRSStatesByUser(ctx context.Context, userID string) ([]domain.FSRSStat, error)
	CountCardsByCardgroupForUser(ctx context.Context, userID string) (map[string]int, error)
}

// statsSwipeRepo is the narrow slice of the swipe-record repository the stats
// aggregate needs to compute the diagnostic performance snapshot over a window.
type statsSwipeRepo interface {
	ListByUserSince(ctx context.Context, userID string, since time.Time) ([]*domain.SwipeRecord, error)
}

// statsCardgroupRepo is the narrow slice of the cardgroup repository the stats
// aggregate needs to answer "does the caller own any deck?" — a count > 0 over
// the caller's cardgroups, independent of whether those decks hold any cards.
type statsCardgroupRepo interface {
	CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error)
}

type statsUsecase struct {
	fsrsRepo      statsFSRSRepo
	swipeRepo     statsSwipeRepo
	cardgroupRepo statsCardgroupRepo
	clock         Clock
}

// NewStats wires the stats usecase from its narrow FSRS + swipe-record +
// cardgroup repository dependencies and an injectable Clock (nil defaults to
// systemClock{} in production; tests inject a fixed Clock so the window/now
// are deterministic). Clock is the same interface NewLearnUsecase uses.
func NewStats(fsrsRepo statsFSRSRepo, swipeRepo statsSwipeRepo, cardgroupRepo statsCardgroupRepo, clock Clock) StatsUsecase {
	if clock == nil {
		clock = systemClock{}
	}
	return &statsUsecase{fsrsRepo: fsrsRepo, swipeRepo: swipeRepo, cardgroupRepo: cardgroupRepo, clock: clock}
}

// LearningStatsResult is the usecase-layer result VO. It carries cardgroup ids
// and struggling-card ids only; the resolver hydrates the Cardgroup / Card
// objects via the DataLoader. The mastery/struggle policy that fills these
// fields lives in the domain services (service.AggregateMastery /
// service.TopStruggling); the usecase only orchestrates the repo reads.
type LearningStatsResult struct {
	Mastery service.MasteryBreakdown
	Decks   []service.DeckMastery
	// OwnsAnyDeck is true iff the caller owns at least one cardgroup, regardless
	// of card count. It lets the client tell a brand-new user (owns nothing) apart
	// from a user who created a deck but has not added cards yet — Decks omits
	// empty decks, so Decks alone cannot make that distinction.
	OwnsAnyDeck     bool
	Performance     service.PerformanceMetrics
	StrugglingCards []service.StrugglingCard
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

	// Mastery half: the global three-tier breakdown plus the per-deck acquisition
	// split — the "mastered" policy — is owned by the domain service; the usecase
	// only forwards the loaded rows.
	mastery, decks, err := service.AggregateMastery(states, totals)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: stats: aggregate mastery")
	}

	// Diagnostic half: a performance snapshot over the trailing statsWindowDays
	// calendar window (so studyStreak reflects real days, not a swipe count) plus
	// the struggling-card ranking derived from the FSRS rows already loaded above.
	now := u.clock.Now()
	swipes, err := u.swipeRepo.ListByUserSince(ctx, caller.Sub, now.AddDate(0, 0, -statsWindowDays))
	if err != nil {
		return nil, eris.Wrap(err, "usecase: stats: list swipes since")
	}

	// Cheap ownership existence check: count > 0 means the caller owns at least
	// one cardgroup, even if all its decks are empty (which totals/Decks omit).
	count, err := u.cardgroupRepo.CountByOwner(ctx, caller.Sub, nil)
	if err != nil {
		return nil, eris.Wrap(err, "usecase: stats: count cardgroups by owner")
	}

	return &LearningStatsResult{
		Mastery:         mastery,
		Decks:           decks,
		OwnsAnyDeck:     count > 0,
		Performance:     service.ComputeMetrics(swipeRecordsByValue(swipes), now),
		StrugglingCards: service.TopStruggling(states, strugglingCardsLimit),
	}, nil
}
