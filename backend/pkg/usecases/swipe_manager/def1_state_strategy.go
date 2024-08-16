package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
	"time"
)

type def1StateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type Def1StateStrategy interface {
	ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error
	IsApplicable(swipeRecords []repository.SwipeRecord) bool
}

func NewDef1StateStrategy(swipeManagerUsecase SwipeManagerUsecase) Def1StateStrategy {
	return &def1StateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

func (d *def1StateStrategy) ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error {
	return d.swipeManagerUsecase.ChangeState(ctx, userID, DEF1)
}

func (d *def1StateStrategy) IsApplicable(swipeRecords []repository.SwipeRecord) bool {
	// Check if none of the conditions match and time since last swipe is significant
	// Implement the specific logic for Def1
	return time.Since(swipeRecords[0].Updated) > 24*time.Hour
}
