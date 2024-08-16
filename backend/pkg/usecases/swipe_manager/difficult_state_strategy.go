package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/services"
	"golang.org/x/net/context"
)

type difficultStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type DifficultStateStrategy interface {
	ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error
	IsApplicable(swipeRecords []repository.SwipeRecord) bool
}

func NewDifficultStateStrategy(swipeManagerUsecase SwipeManagerUsecase) DifficultStateStrategy {
	return &difficultStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

func (d *difficultStateStrategy) ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error {
	return d.swipeManagerUsecase.ChangeState(ctx, userID, DIFFICULT)
}

func (d *difficultStateStrategy) IsApplicable(swipeRecords []repository.SwipeRecord) bool {
	// If the last 5 records indicate other than "known", configure difficult
	unknownCount := 0
	for i := 0; i < 5 && i < len(swipeRecords); i++ {
		if swipeRecords[i].Direction != services.KNOWN {
			unknownCount++
		}
	}
	return unknownCount == 5
}
