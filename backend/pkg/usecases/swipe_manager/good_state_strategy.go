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

// GoodStateStrategy implements the SwipeManagerUsecase interface
type goodStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
	amountOfSwipes      int
}

type GoodStateStrategy interface {
	SwipeStrategy
}

// NewGoodStateStrategy returns an instance of GoodStateStrategy
func NewGoodStateStrategy(swipeManagerUsecase SwipeManagerUsecase) GoodStateStrategy {
	return &goodStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
		amountOfSwipes:      config.Cfg.FLBatchDefaultAmount,
	}
}

// Run ChangeState changes the state of the given swipe records to GOOD
func (g *goodStateStrategy) Run(ctx context.Context,
	newSwipeRecord model.NewSwipeRecord) ([]*model.Card, error) {

	// Default algorithm, random words updated old, but the interval is closer
	cards, err := g.swipeManagerUsecase.Srv().GetRandomCardsFromRecentUpdates(
		ctx,
		newSwipeRecord.CardGroupID,
		config.Cfg.PGQueryLimit,
		repo.ASC,
		repo.ASC)

	if err != nil {
		return nil, goerr.Wrap(err)
	}

	cardAmount, err := g.swipeManagerUsecase.DetermineCardAmount(
		cards,
		g.amountOfSwipes)

	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards[:cardAmount], nil
}

func (g *goodStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord, latestSwipeRecords []*repository.SwipeRecord) bool {
	// It needs to be certain amount of data for this mode.
	if len(latestSwipeRecords) < g.amountOfSwipes {
		return false
	}

	// Check if 5 out of the last 10 records are "known"
	knownCount := 0
	for i := 0; i < g.amountOfSwipes && i < len(latestSwipeRecords); i++ {
		if latestSwipeRecords[i].Mode == services.KNOWN {
			knownCount++
		}
	}

	mode := knownCount >= 5
	if mode {
		logger.Logger.Debug("Good mode")
	}
	return mode
}
