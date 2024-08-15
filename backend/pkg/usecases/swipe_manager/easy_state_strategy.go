package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
)

type EasyStateStrategy struct{}

func (e EasyStateStrategy) ChangeState(ctx context.Context, s *SwipeManagerUsecase, swipeRecords []repository.SwipeRecord) error {
	return s.changeState(ctx, userID, EASY)
}
