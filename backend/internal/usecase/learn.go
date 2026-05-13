package usecase

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

const (
	defaultLearnNextDueLimit = 20
	maxLearnNextDueLimit     = 100
)

type CardRepoForLearn interface {
	FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error)
}

type CardgroupRepoForLearn interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

type LearnUsecase struct {
	cardRepo      CardRepoForLearn
	cardgroupRepo CardgroupRepoForLearn
	ordering      *service.OrderingPolicy
	randSource    func() *rand.Rand
	defaultLimit  int
	maxLimit      int
}

func NewLearnUsecase(
	cardRepo CardRepoForLearn,
	cardgroupRepo CardgroupRepoForLearn,
	ordering *service.OrderingPolicy,
	randSource func() *rand.Rand,
	defaultLimit, maxLimit int,
) *LearnUsecase {
	if ordering == nil {
		ordering = service.NewOrderingPolicy()
	}
	if randSource == nil {
		randSource = func() *rand.Rand {
			return rand.New(rand.NewSource(time.Now().UnixNano()))
		}
	}
	if defaultLimit <= 0 {
		defaultLimit = defaultLearnNextDueLimit
	}
	if maxLimit <= 0 {
		maxLimit = maxLearnNextDueLimit
	}
	if cardRepo == nil {
		panic("LearnUsecase: cardRepo must not be nil")
	}
	if cardgroupRepo == nil {
		panic("LearnUsecase: cardgroupRepo must not be nil")
	}
	if defaultLimit > maxLimit {
		panic(fmt.Sprintf("LearnUsecase: defaultLimit (%d) must not exceed maxLimit (%d)", defaultLimit, maxLimit))
	}
	return &LearnUsecase{
		cardRepo:      cardRepo,
		cardgroupRepo: cardgroupRepo,
		ordering:      ordering,
		randSource:    randSource,
		defaultLimit:  defaultLimit,
		maxLimit:      maxLimit,
	}
}

// NextDueCards returns up to limit due cards (clamped to [1, maxLimit]; now must be UTC).
// Returns Unauthenticated when the caller does not own the cardgroup, BadUserInput when the cardgroup is missing.
func (u *LearnUsecase) NextDueCards(ctx context.Context, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	cg, err := u.cardgroupRepo.FindByID(ctx, cardgroupID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, gqlerr.BadUserInput("cardgroupId", "cardgroup not found")
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	if cg.OwnerID != user.Sub {
		return nil, gqlerr.Unauthenticated()
	}
	limit = u.clampLimit(limit)
	cards, err := u.cardRepo.FindDueCardsForUser(ctx, user.Sub, cardgroupID, now, limit)
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return u.ordering.Apply(cards, u.randSource()), nil
}

func (u *LearnUsecase) clampLimit(limit int) int {
	if limit <= 0 {
		return u.defaultLimit
	}
	if limit > u.maxLimit {
		return u.maxLimit
	}
	return limit
}
