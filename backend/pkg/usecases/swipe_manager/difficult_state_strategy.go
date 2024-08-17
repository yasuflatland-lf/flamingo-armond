package swipe_manager

import (
	"backend/graph/model"
	"golang.org/x/net/context"
)

type difficultStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type DifficultStateStrategy interface {
	SwipeStrategy
}

func NewDifficultStateStrategy(swipeManagerUsecase SwipeManagerUsecase) DifficultStateStrategy {
	return &difficultStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

func (d *difficultStateStrategy) ChangeState(ctx context.Context, newSwipeRecord model.NewSwipeRecord) error {
	return d.swipeManagerUsecase.ChangeState(ctx, newSwipeRecord.CardGroupID, newSwipeRecord.UserID, DIFFICULT)
	return nil, nil
}

func (d *difficultStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord) bool {
	//// If the last 5 records indicate other than "known", configure difficult
	//unknownCount := 0
	//for i := 0; i < 5 && i < len(swipeRecords); i++ {
	//	if swipeRecords[i].Direction != services.KNOWN {
	//		unknownCount++
	//	}
	//}
	//return unknownCount == 5
	return true
}
