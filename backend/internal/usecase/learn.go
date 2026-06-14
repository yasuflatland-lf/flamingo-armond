package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

const (
	defaultLearnNextDueLimit = 20
	maxLearnNextDueLimit     = 100
)

// jstZone is the fixed UTC+9 offset used to compute the learner's
// start-of-day boundary. JST observes no daylight saving, so a fixed offset
// is exact and avoids a tzdata dependency. The product currently assumes a
// Japan-resident learner; promote to a per-user preference if that breaks.
var jstZone = time.FixedZone("JST", 9*60*60)

// startOfDayJST returns the JST midnight at or before now, as an absolute
// instant. The learn queue's review slots include only cards whose last_review
// is strictly before this boundary, so a card swiped today never re-enters
// today's queue regardless of its FSRS re-due interval.
func startOfDayJST(now time.Time) time.Time {
	local := now.In(jstZone)
	y, m, d := local.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, jstZone)
}

type CardRepoForLearn interface {
	FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, now, reviewedBefore time.Time, limit int) ([]domain.DueCard, error)
	// FindPracticeCardsForUser returns cards the user reviewed today (the inverse
	// window of FindDueCardsForUser): last_review at or after reviewedAfter. The
	// server randomizes row order; the usecase preserves it verbatim.
	FindPracticeCardsForUser(ctx context.Context, userID, cardgroupID string, reviewedAfter time.Time, limit int) ([]domain.DueCard, error)
}

type CardgroupRepoForLearn interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

// LearnUsecase surfaces due-card retrieval for a learning session.
type LearnUsecase interface {
	NextDueCards(ctx context.Context, cardgroupID string, limit *int) ([]*domain.Card, error)
	// PracticeTodaysCards returns the cards the caller already reviewed today
	// (JST), the inverse window of NextDueCards. It is read-only: no FSRS
	// schedule ordering is applied and nothing is written. Returns Unauthenticated
	// when the caller does not own the cardgroup, BadUserInput when the cardgroup is missing.
	PracticeTodaysCards(ctx context.Context, cardgroupID string, limit *int) ([]*domain.Card, error)
}

type learnUsecase struct {
	cardRepo      CardRepoForLearn
	cardgroupRepo CardgroupRepoForLearn
	ordering      *service.OrderingPolicy
	randSource    func() *rand.Rand
	clock         Clock
	defaultLimit  int
	maxLimit      int
	logger        *slog.Logger
}

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func NewLearnUsecase(
	cardRepo CardRepoForLearn,
	cardgroupRepo CardgroupRepoForLearn,
	ordering *service.OrderingPolicy,
	randSource func() *rand.Rand,
	defaultLimit, maxLimit int,
	clock Clock,
	logger *slog.Logger,
) LearnUsecase {
	if logger == nil {
		panic("usecase: learn: logger is required")
	}
	if cardRepo == nil {
		panic("usecase: learn: cardRepo must not be nil")
	}
	if cardgroupRepo == nil {
		panic("usecase: learn: cardgroupRepo must not be nil")
	}
	if ordering == nil {
		ordering = service.NewOrderingPolicy()
	}
	if randSource == nil {
		randSource = func() *rand.Rand {
			return rand.New(rand.NewSource(time.Now().UnixNano()))
		}
	}
	if clock == nil {
		clock = systemClock{}
	}
	if defaultLimit <= 0 {
		defaultLimit = defaultLearnNextDueLimit
	}
	if maxLimit <= 0 {
		maxLimit = maxLearnNextDueLimit
	}
	if defaultLimit > maxLimit {
		panic(fmt.Sprintf("usecase: learn: defaultLimit (%d) must not exceed maxLimit (%d)", defaultLimit, maxLimit))
	}
	return &learnUsecase{
		cardRepo:      cardRepo,
		cardgroupRepo: cardgroupRepo,
		ordering:      ordering,
		randSource:    randSource,
		clock:         clock,
		defaultLimit:  defaultLimit,
		maxLimit:      maxLimit,
		logger:        logger,
	}
}

// authorizeCardgroupForLearn resolves the cardgroup and verifies the caller owns
// it. It is shared by NextDueCards and PracticeTodaysCards so the two windows
// cannot drift in how they translate auth and cardgroup-lookup failures.
// Returns the authenticated user on success; the cardgroup itself is discarded
// (callers only need the ownership decision and the user's Sub).
func (u *learnUsecase) authorizeCardgroupForLearn(ctx context.Context, cardgroupID string) (*auth.AuthUser, error) {
	user := auth.UserFrom(ctx)
	if err := requireCallerSub(user); err != nil {
		return nil, err
	}
	cg, err := u.cardgroupRepo.FindByID(ctx, cardgroupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ucerr.NewValidationError("cardgroupId", "cardgroup not found")
		}
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: find cardgroup by id")
	}
	if !cg.IsOwnedBy(domain.UserID(user.Sub)) {
		return nil, ucerr.ErrUnauthenticated
	}
	return user, nil
}

// NextDueCards returns up to limit due cards (clamped to [1, maxLimit]; a nil or non-positive limit falls back to defaultLimit).
// Returns Unauthenticated when the caller does not own the cardgroup, BadUserInput when the cardgroup is missing.
func (u *learnUsecase) NextDueCards(ctx context.Context, cardgroupID string, limit *int) ([]*domain.Card, error) {
	user, err := u.authorizeCardgroupForLearn(ctx, cardgroupID)
	if err != nil {
		return nil, err
	}
	n := 0
	if limit != nil {
		n = *limit
	}
	n = u.clampLimit(n)
	now := u.clock.Now().UTC()
	due, err := u.cardRepo.FindDueCardsForUser(ctx, user.Sub, cardgroupID, now, startOfDayJST(now), n)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: learn: find due cards")
	}
	ordered := u.ordering.Apply(due, u.randSource())
	if len(ordered) > n {
		ordered = ordered[:n]
	}
	return ordered, nil
}

// PracticeTodaysCards returns the cards the caller already reviewed today (JST),
// the inverse window of NextDueCards. It is strictly read-only: it never invokes
// OrderingPolicy (practice replay is not schedule ordering) and never writes.
// The repository randomizes row order server-side; this method preserves that
// order verbatim, mapping DueCards to their underlying *domain.Card.
func (u *learnUsecase) PracticeTodaysCards(ctx context.Context, cardgroupID string, limit *int) ([]*domain.Card, error) {
	user, err := u.authorizeCardgroupForLearn(ctx, cardgroupID)
	if err != nil {
		return nil, err
	}
	n := 0
	if limit != nil {
		n = *limit
	}
	n = u.clampPracticeLimit(n)
	now := u.clock.Now().UTC()
	boundary := startOfDayJST(now)
	rows, err := u.cardRepo.FindPracticeCardsForUser(ctx, user.Sub, cardgroupID, boundary, n)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: learn: find practice cards")
	}
	// Preserve repository order (already randomized server-side); do not apply
	// OrderingPolicy and do not truncate beyond the clamp. Empty stays non-nil.
	cards := make([]*domain.Card, 0, len(rows))
	for _, row := range rows {
		cards = append(cards, row.Card)
	}
	return cards, nil
}

func (u *learnUsecase) clampLimit(limit int) int {
	if limit <= 0 {
		return u.defaultLimit
	}
	if limit > u.maxLimit {
		return u.maxLimit
	}
	return limit
}

// clampPracticeLimit clamps the practice-mode limit. Unlike clampLimit, the
// default IS the cap: a missing or non-positive limit yields the cap
// (u.maxLimit), because the unit of practice is the entire set of cards
// reviewed today, not a paged subset. The deliberate asymmetry with
// clampLimit's default-20 is why this is a separate named method.
func (u *learnUsecase) clampPracticeLimit(limit int) int {
	if limit <= 0 {
		return u.maxLimit
	}
	if limit > u.maxLimit {
		return u.maxLimit
	}
	return limit
}
