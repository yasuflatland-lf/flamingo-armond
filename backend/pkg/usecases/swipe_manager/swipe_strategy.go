package swipe_manager

import (
	repository "backend/graph/db"
	"github.com/m-mizutani/goerr"
	"golang.org/x/net/context"
)

type SwipeStrategy interface {
	ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error
	IsApplicable(records []repository.SwipeRecord) bool
}

type strategyExecutor struct {
	strategy SwipeStrategy
}

type StrategyExecutor interface {
	ExecuteStrategy(ctx context.Context, swipeRecords []repository.SwipeRecord) ([]repository.Card, error)
}

func NewStrategyExecutor(strategy SwipeStrategy) StrategyExecutor {
	return &strategyExecutor{
		strategy: strategy,
	}
}

func (sc *strategyExecutor) ExecuteStrategy(ctx context.Context, swipeRecords []repository.SwipeRecord) ([]repository.Card, error) {
	err := sc.strategy.ChangeState(ctx, swipeRecords)
	if err != nil {
		return nil, goerr.Wrap(err)
	}
	return nil, nil
}
