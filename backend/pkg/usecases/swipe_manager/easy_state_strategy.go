package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/services"
	"golang.org/x/net/context"
)

type easyStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type EasyStateStrategy interface {
	ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error
	IsApplicable(swipeRecords []repository.SwipeRecord) bool
}

func NewEasyStateStrategy(swipeManagerUsecase SwipeManagerUsecase) EasyStateStrategy {
	return &easyStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

func (e *easyStateStrategy) ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error {
	// Assuming userID and EASY are available in this context
	return e.swipeManagerUsecase.ChangeState(ctx, userID, EASY)
}

func (e *easyStateStrategy) IsApplicable(swipeRecords []repository.SwipeRecord) bool {
	// Check if the last 5 records indicate "known"
	knownCount := 0
	for i := 0; i < 5 && i < len(swipeRecords); i++ {
		if swipeRecords[i].Direction == services.KNOWN {
			knownCount++
		}
	}
	return knownCount == 5
}
