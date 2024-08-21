package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/pkg/config"
	"backend/pkg/logger"
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

func (d *defaultStateStrategy) Run(ctx context.Context,
	newSwipeRecord model.NewSwipeRecord) ([]*model.Card, error) {

	// Fetch random recent added words
	cards, err := d.swipeManagerUsecase.Srv().GetRandomCardsFromRecentUpdates(ctx, newSwipeRecord.CardGroupID, config.Cfg.PGQueryLimit, repo.DESC, repo.ASC)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	cardAmount, err := d.swipeManagerUsecase.DetermineCardAmount(cards, d.amountOfRecentRandomWords)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards[:cardAmount], nil
}

func (d *defaultStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord, latestSwipeRecords []*repository.SwipeRecord) bool {
	// Default Strategy must be hit no matter what
	logger.Logger.Debug("Default mode")
	return true
}
