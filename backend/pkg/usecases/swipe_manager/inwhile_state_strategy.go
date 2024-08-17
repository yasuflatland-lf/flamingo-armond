package swipe_manager

import (
	"backend/graph/model"
	"golang.org/x/net/context"
	"time"
)

type inWhileStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type InWhileStateStrategy interface {
	SwipeStrategy
}

func NewInWhileStateStrategy(swipeManagerUsecase SwipeManagerUsecase) InWhileStateStrategy {
	return &inWhileStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

func (d *inWhileStateStrategy) Run(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	d.swipeManagerUsecase.ChangeState(ctx, newSwipeRecord.CardGroupID, newSwipeRecord.UserID, INWHILE)
	return nil, nil
}

func (d *inWhileStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord) bool {
	// Check if none of the conditions match and time since last swipe is significant
	// Implement the specific logic for Def1
	return time.Since(newSwipeRecord.Updated) > 24*time.Hour
}
