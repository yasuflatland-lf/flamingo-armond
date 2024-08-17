package swipe_manager

import (
	"backend/graph/model"
	"golang.org/x/net/context"
)

type easyStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type EasyStateStrategy interface {
	SwipeStrategy
}

func NewEasyStateStrategy(swipeManagerUsecase SwipeManagerUsecase) EasyStateStrategy {
	return &easyStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

func (e *easyStateStrategy) Run(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	// Assuming userID and EASY are available in this context
	e.swipeManagerUsecase.ChangeState(ctx, newSwipeRecord.CardGroupID, newSwipeRecord.UserID, EASY)

	return nil, nil
}

func (e *easyStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord) bool {
	//// Check if the last 5 records indicate "known"
	//knownCount := 0
	//for i := 0; i < 5 && i < len(swipeRecords); i++ {
	//	if swipeRecords[i].Direction == services.KNOWN {
	//		knownCount++
	//	}
	//}
	//return knownCount == 5
	return true
}
