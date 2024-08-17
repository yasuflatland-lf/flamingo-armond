package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
)

type SwipeStrategy interface {
	ChangeState(ctx context.Context, latestSwipeRecord repository.SwipeRecord) error
	IsApplicable(ctx context.Context, latestSwipeRecord repository.SwipeRecord) bool
}

type strategyExecutor struct {
	strategy SwipeStrategy
}

type StrategyExecutor interface {
	ExecuteStrategy(ctx context.Context, latestSwipeRecord repository.SwipeRecord) ([]repository.Card, error)
}

func NewStrategyExecutor(strategy SwipeStrategy) StrategyExecutor {
	return &strategyExecutor{
		strategy: strategy,
	}
}

func (sc *strategyExecutor) ExecuteStrategy(ctx context.Context, latestSwipeRecord repository.SwipeRecord) ([]repository.Card, error) {
	return nil, nil
}
