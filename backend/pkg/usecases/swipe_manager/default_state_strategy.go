package swipe_manager

import (
	"backend/graph/model"
	"backend/pkg/config"
	repo "backend/pkg/repository"
	"github.com/m-mizutani/goerr"
	"golang.org/x/net/context"
)

type defaultStateStrategy struct {
	swipeManagerUsecase       SwipeManagerUsecase
	amountOfRecentRandomWords int
}

type DefaultStateStrategy interface {
	SwipeStrategy
}

func NewDefaultStateStrategy(
	swipeManagerUsecase SwipeManagerUsecase) DefaultStateStrategy {
	return &defaultStateStrategy{
		swipeManagerUsecase:       swipeManagerUsecase,
		amountOfRecentRandomWords: config.Cfg.FLBatchDefaultAmount,
	}
}

func (d *defaultStateStrategy) Run(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	d.swipeManagerUsecase.ChangeState(ctx, newSwipeRecord.CardGroupID, newSwipeRecord.UserID, DEFAULT)

	// Fetch random recent added words
	cards, err := d.swipeManagerUsecase.Srv().GetRandomCardsFromRecentUpdates(ctx, newSwipeRecord.CardGroupID, config.Cfg.PGQueryLimit, repo.DESC, repo.ASC)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards[:d.amountOfRecentRandomWords], nil
}

func (d *defaultStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord) bool {
	// Default Strategy must be hit no matter what
	return true
}
