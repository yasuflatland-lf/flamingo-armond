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

type CardRepoForLearn interface {
	FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, now time.Time, limit int) ([]domain.DueCard, error)
}

type CardgroupRepoForLearn interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

// LearnUsecase surfaces due-card retrieval for a learning session.
type LearnUsecase interface {
	NextDueCards(ctx context.Context, cardgroupID string, limit *int) ([]*domain.Card, error)
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

// NextDueCards returns up to limit due cards (clamped to [1, maxLimit]).
// Returns Unauthenticated when the caller does not own the cardgroup, BadUserInput when the cardgroup is missing.
func (u *learnUsecase) NextDueCards(ctx context.Context, cardgroupID string, limit *int) ([]*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, ucerr.ErrUnauthenticated
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
	if !cg.IsOwnedBy(user.Sub) {
		return nil, ucerr.ErrUnauthenticated
	}
	n := 0
	if limit != nil {
		n = *limit
	}
	n = u.clampLimit(n)
	now := u.clock.Now().UTC()
	due, err := u.cardRepo.FindDueCardsForUser(ctx, user.Sub, cardgroupID, now, n)
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

func (u *learnUsecase) clampLimit(limit int) int {
	if limit <= 0 {
		return u.defaultLimit
	}
	if limit > u.maxLimit {
		return u.maxLimit
	}
	return limit
}
