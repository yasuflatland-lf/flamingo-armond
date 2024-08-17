package swipe_manager

import (
	"backend/graph/model"
	"golang.org/x/net/context"
)

type defaultStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type DefaultStateStrategy interface {
	SwipeStrategy
}

func NewDefaultStateStrategy(swipeManagerUsecase SwipeManagerUsecase) DefaultStateStrategy {
	return &defaultStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

func (d *defaultStateStrategy) Run(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	// Assuming userID is somehow available in the context or swipeRecords
	d.swipeManagerUsecase.ChangeState(ctx, newSwipeRecord.CardGroupID, newSwipeRecord.UserID, DEFAULT)

	return nil, nil
}

func (d *defaultStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord) bool {
	// Default Strategy must be hit no matter what
	return true
}
