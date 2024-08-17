package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
)

// GoodStateStrategy implements the SwipeManagerUsecase interface
type goodStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type GoodStateStrategy interface {
	SwipeStrategy
}

// NewGoodStateStrategy returns an instance of GoodStateStrategy
func NewGoodStateStrategy(swipeManagerUsecase SwipeManagerUsecase) GoodStateStrategy {
	return &goodStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

// ChangeState changes the state of the given swipe records to GOOD
func (g *goodStateStrategy) ChangeState(ctx context.Context, latestSwipeRecord repository.SwipeRecord) error {
	return g.swipeManagerUsecase.ChangeState(ctx, latestSwipeRecord.CardGroupID, latestSwipeRecord.UserID, GOOD)
}

func (g *goodStateStrategy) IsApplicable(ctx context.Context, latestSwipeRecord repository.SwipeRecord) bool {
	//// Check if 5 out of the last 10 records are "known"
	//knownCount := 0
	//for i := 0; i < 10 && i < len(swipeRecords); i++ {
	//	if swipeRecords[i].Direction == services.KNOWN {
	//		knownCount++
	//	}
	//}
	//return knownCount >= 5
	return true
}
