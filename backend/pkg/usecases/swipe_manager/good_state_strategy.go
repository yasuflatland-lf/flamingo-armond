package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
)

type GoodStateStrategy struct{}

func (g GoodStateStrategy) ChangeState(ctx context.Context, s *SwipeManagerUsecase, swipeRecords []repository.SwipeRecord) error {
	return s.changeState(ctx, userID, GOOD)
}
