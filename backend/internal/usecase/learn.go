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
)

const (
	defaultLearnNextDueLimit = 20
	maxLearnNextDueLimit     = 100
)

type CardRepoForLearn interface {
	// FindDueCardsForUser receives both ends of the current JST learn day. Rescue
	// reviews use rescueDueBefore; filler reviews use now; both exclude rows
	// reviewed at or after reviewedBefore. rescueReviewedBefore additionally
	// floors the rescue window's early serve at a whole day of elapsed time.
	FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, now, reviewedBefore, rescueDueBefore, rescueReviewedBefore time.Time, limit int) ([]domain.DueCard, error)
	// FindPracticeCardsForUser returns cards the user reviewed today (the inverse
	// window of FindDueCardsForUser): last_review at or after reviewedAfter. The
	// server randomizes row order; the usecase preserves it verbatim.
	FindPracticeCardsForUser(ctx context.Context, userID, cardgroupID string, reviewedAfter time.Time, limit int) ([]domain.DueCard, error)
}

type CardgroupRepoForLearn interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

// UserPrefsForLearn is the narrow consumer interface for reading the caller's
// stored preferences (the per-user new-card ratio). Satisfied by
// repository.UserPreferenceRepository. ErrNotFound (no row) and a nil pref fall
// back to domain.DefaultNewCardRatio at the read site.
type UserPrefsForLearn interface {
	FindByUserID(ctx context.Context, userID string) (*domain.UserPreference, error)
}

// LearnUsecase surfaces due-card retrieval for a learning session.
type LearnUsecase interface {
	NextDueCards(ctx context.Context, cardgroupID string, limit *int) ([]*domain.Card, error)
	// PracticeTodaysCards returns the cards the caller already reviewed today
	// (JST), the inverse window of NextDueCards. It is read-only: no FSRS
	// schedule ordering is applied and nothing is written. Returns Unauthenticated
	// when the caller does not own the cardgroup, BadUserInput when the cardgroup is missing.
	PracticeTodaysCards(ctx context.Context, cardgroupID string, limit *int) ([]*domain.Card, error)
	// DefaultIfNew returns ucs unchanged when the user already has a scheduling
	// record for the card, otherwise the default new-card FSRS state. "A missing
	// FSRS record means a brand-new card with default state" is an application
	// policy; this method keeps that decision out of the presentation layer while
	// the resolver retains the DataLoader batch that produces the nil input.
	DefaultIfNew(ucs *domain.UserCardFSRS, userID domain.UserID, cardID string, createdAt time.Time) *domain.UserCardFSRS
}

type learnUsecase struct {
	cardRepo      CardRepoForLearn
	cardgroupRepo CardgroupRepoForLearn
	userPrefs     UserPrefsForLearn
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
	userPrefs UserPrefsForLearn,
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
	if userPrefs == nil {
		panic("usecase: learn: userPrefs must not be nil")
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
		userPrefs:     userPrefs,
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
	if err := authorizeCardgroupOrBadInput(ctx, u.cardgroupRepo, domain.CardgroupID(cardgroupID), domain.UserID(user.Sub)); err != nil {
		return nil, err
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
	// domain.RescueReviewedBefore(now) is the minimum-elapsed floor for the
	// rescue window: a card repeated inside the same 24 hours earns zero FSRS
	// scheduling credit, so it must not be served early.
	due, err := u.cardRepo.FindDueCardsForUser(ctx, user.Sub, cardgroupID, now, domain.StartOfLearnDay(now), domain.EndOfLearnDay(now), domain.RescueReviewedBefore(now), n)
	if err != nil {
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: learn: find due cards")
	}
	// Load the caller's per-user new-vs-review ratio. A missing preference row
	// (ErrNotFound), a nil pref, or a zero-value ratio all fall back to the
	// default; a real infrastructure error propagates (context-done unwrapped).
	// EffectiveNewCardRatio owns the zero-to-default resolution.
	ratio := domain.DefaultNewCardRatio
	pref, err := u.userPrefs.FindByUserID(ctx, user.Sub)
	switch {
	case err == nil && pref != nil:
		ratio = pref.EffectiveNewCardRatio()
	case err != nil && !errors.Is(err, repository.ErrNotFound):
		if isContextDone(err) {
			return nil, err
		}
		return nil, eris.Wrap(err, "usecase: learn: load user preference")
	}
	ordered := u.ordering.Apply(due, u.randSource(), ratio)
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
	boundary := domain.StartOfLearnDay(now)
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

// DefaultIfNew implements the new-card FSRS default policy: a nil scheduling
// record (the DataLoader's documented contract for a card the user has never
// seen) is synthesized into the default new-card state; a non-nil record passes
// through unchanged. The resolver calls this after its batched Load so the
// "missing record means default state" decision lives in the application layer.
func (u *learnUsecase) DefaultIfNew(ucs *domain.UserCardFSRS, userID domain.UserID, cardID string, createdAt time.Time) *domain.UserCardFSRS {
	if ucs != nil {
		return ucs
	}
	return domain.NewUserCardFSRSForNewCard(userID, cardID, createdAt)
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
