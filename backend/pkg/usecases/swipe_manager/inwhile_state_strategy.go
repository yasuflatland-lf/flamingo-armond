package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
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

func (d *inWhileStateStrategy) ChangeState(ctx context.Context, latestSwipeRecord repository.SwipeRecord) error {
	return d.swipeManagerUsecase.ChangeState(ctx, latestSwipeRecord.CardGroupID, latestSwipeRecord.UserID, INWHILE)
}

func (d *inWhileStateStrategy) IsApplicable(ctx context.Context, latestSwipeRecord repository.SwipeRecord) bool {
	//// Check if none of the conditions match and time since last swipe is significant
	//// Implement the specific logic for Def1
	//return time.Since(swipeRecords[0].Updated) > 24*time.Hour
	return true
}
