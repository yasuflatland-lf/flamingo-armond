package swipe_manager

import (
	"backend/graph/model"
	"backend/pkg/config"
	repo "backend/pkg/repository"
	"github.com/m-mizutani/goerr"
	"golang.org/x/net/context"
	"time"
)

type inWhileStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
	amountOfKnownWords  int
}

type InWhileStateStrategy interface {
	SwipeStrategy
}

func NewInWhileStateStrategy(swipeManagerUsecase SwipeManagerUsecase) InWhileStateStrategy {
	return &inWhileStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
		amountOfKnownWords:  config.Cfg.FLBatchDefaultAmount,
	}
}

func (d *inWhileStateStrategy) Run(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	d.swipeManagerUsecase.ChangeState(ctx, newSwipeRecord.CardGroupID, newSwipeRecord.UserID, INWHILE)

	// Fetch random known words
	cards, err := d.swipeManagerUsecase.Srv().GetRandomCardsFromRecentUpdates(ctx, newSwipeRecord.CardGroupID, config.Cfg.PGQueryLimit, repo.DESC, repo.DESC)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards[:d.amountOfKnownWords], nil
}

func (d *inWhileStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord) bool {
	// If the last swipe was a week ago or before.
	return time.Since(newSwipeRecord.Updated) > 168*time.Hour
}
