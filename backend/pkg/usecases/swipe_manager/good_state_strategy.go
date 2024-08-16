package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/services"
	"golang.org/x/net/context"
)

// GoodStateStrategy implements the SwipeManagerUsecase interface
type goodStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
}

type GoodStateStrategy interface {
	ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error
	IsApplicable(swipeRecords []repository.SwipeRecord) bool
}

// NewGoodStateStrategy returns an instance of GoodStateStrategy
func NewGoodStateStrategy(swipeManagerUsecase SwipeManagerUsecase) GoodStateStrategy {
	return &goodStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
	}
}

// ChangeState changes the state of the given swipe records to GOOD
func (g *goodStateStrategy) ChangeState(ctx context.Context, swipeRecords []repository.SwipeRecord) error {
	return g.swipeManagerUsecase.ChangeState(ctx, userID, GOOD)
}

func (g *goodStateStrategy) IsApplicable(swipeRecords []repository.SwipeRecord) bool {
	// Check if 5 out of the last 10 records are "known"
	knownCount := 0
	for i := 0; i < 10 && i < len(swipeRecords); i++ {
		if swipeRecords[i].Direction == services.KNOWN {
			knownCount++
		}
	}
	return knownCount >= 5
}
