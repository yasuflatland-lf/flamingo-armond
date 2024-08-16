package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
)

type def2StateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type Def2StateStrategy interface {
	ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error
	IsApplicable(swipeRecords []repository.SwipeRecord) bool
}

// NewDef2StateStrategy creates a new instance of Def2StateStrategy.
func NewDef2StateStrategy(swipeManagerUsecase SwipeManagerUsecase) Def2StateStrategy {
	return &def2StateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

// ChangeState changes the state of the swipe records to DEF2.
func (d *def2StateStrategy) ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error {
	// Assuming userID is somehow available in the context or swipeRecords
	return d.swipeManagerUsecase.ChangeState(ctx, userID, DEF2)
}

func (d *def2StateStrategy) IsApplicable(swipeRecords []repository.SwipeRecord) bool {
	return true
}
