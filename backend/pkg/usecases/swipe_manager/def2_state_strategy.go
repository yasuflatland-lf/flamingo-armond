package swipe_manager

import (
	repository "backend/graph/db"
	"golang.org/x/net/context"
)

type Def2StateStrategy struct{}

func (d Def2StateStrategy) ChangeState(ctx context.Context, s *SwipeManagerUsecase, swipeRecords []repository.SwipeRecord) error {
	return s.changeState(ctx, userID, DEF2)
}
