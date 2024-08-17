package swipe_manager

import (
	"backend/graph/model"
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

// Run ChangeState changes the state of the given swipe records to GOOD
func (g *goodStateStrategy) Run(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	g.swipeManagerUsecase.ChangeState(ctx, newSwipeRecord.CardGroupID, newSwipeRecord.UserID, GOOD)

	return nil, nil
}

func (g *goodStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord) bool {
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
