package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
)

type StateStrategy interface {
	ChangeState(ctx context.Context, s *SwipeManagerUsecase, swipeRecords []repository.SwipeRecord) error
}

type StateContext struct {
	strategy StateStrategy
}

func (sc *StateContext) SetStrategy(strategy StateStrategy) {
	sc.strategy = strategy
}

func (sc *StateContext) ExecuteStrategy(ctx context.Context, s *SwipeManagerUsecase, swipeRecords []repository.SwipeRecord) ([]repository.Card, error) {
	sc.strategy.ChangeState(ctx, s, swipeRecords)
	return nil, nil
}
