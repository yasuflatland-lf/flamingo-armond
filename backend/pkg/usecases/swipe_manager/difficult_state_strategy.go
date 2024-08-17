package swipe_manager

import (
	repository "backend/graph/db"
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

func (d *difficultStateStrategy) ChangeState(ctx context.Context, latestSwipeRecord repository.SwipeRecord) error {
	return d.swipeManagerUsecase.ChangeState(ctx, latestSwipeRecord.CardGroupID, latestSwipeRecord.UserID, DIFFICULT)
}

func (d *difficultStateStrategy) IsApplicable(ctx context.Context, latestSwipeRecord repository.SwipeRecord) bool {
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
