package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/config"
	"backend/pkg/logger"
	repo "backend/pkg/repository"
	"github.com/m-mizutani/goerr"
	"golang.org/x/net/context"
)

type easyStateStrategy struct {
	swipeManagerUsecase   SwipeManagerUsecase
	amaountOfUnKnownWords int
}

type EasyStateStrategy interface {
	SwipeStrategy
}

func NewEasyStateStrategy(swipeManagerUsecase SwipeManagerUsecase) EasyStateStrategy {
	return &easyStateStrategy{
		swipeManagerUsecase:   swipeManagerUsecase,
		amaountOfUnKnownWords: config.Cfg.FLBatchDefaultAmount,
	}
}

func (e *easyStateStrategy) Run(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {

	// Fetch random unknown words
	cards, err := e.swipeManagerUsecase.Srv().GetRandomCardsFromRecentUpdates(ctx, newSwipeRecord.CardGroupID, config.Cfg.PGQueryLimit, repo.ASC, repo.ASC)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	cardAmount, err := e.swipeManagerUsecase.DetermineCardAmount(cards, e.amaountOfUnKnownWords)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards[:cardAmount], nil
}

func (e *easyStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord, latestSwipeRecords []*repository.SwipeRecord) bool {
	// Check if the last 5 records indicate "easy"
	knownCount := 0
	for i := 0; i < 5 && i < len(latestSwipeRecords); i++ {
		if latestSwipeRecords[i].Mode == services.KNOWN {
			knownCount++
		}
	}

	mode := knownCount == 5
	if mode {
		logger.Logger.Debug("Easy mode")
	}
	return mode
}
