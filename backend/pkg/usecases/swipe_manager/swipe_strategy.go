package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"golang.org/x/net/context"
)

type SwipeStrategy interface {
	Run(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error)
	IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord, latestSwipeRecords []*repository.SwipeRecord) bool
}
