package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
)

type defaultStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type DefaultStateStrategy interface {
	SwipeStrategy
}

// NewDefaultStateStrategy creates a new instance of Def2StateStrategy.
func NewDefaultStateStrategy(swipeManagerUsecase SwipeManagerUsecase) DefaultStateStrategy {
	return &defaultStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

func (d *defaultStateStrategy) ChangeState(ctx context.Context, latestSwipeRecord repository.SwipeRecord) error {
	// Assuming userID is somehow available in the context or swipeRecords
	return d.swipeManagerUsecase.ChangeState(ctx, latestSwipeRecord.CardGroupID, latestSwipeRecord.UserID, DEFAULT)
}

func (d *defaultStateStrategy) IsApplicable(ctx context.Context, latestSwipeRecord repository.SwipeRecord) bool {
	return true
}
