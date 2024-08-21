package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/pkg/config"
	"backend/pkg/logger"
	repo "backend/pkg/repository"
	"fmt"
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

func (d *inWhileStateStrategy) Run(ctx context.Context,
	newSwipeRecord model.NewSwipeRecord) ([]*model.Card, error) {
	// Fetch random known words, sorting by the most recent updates
	cards, err := d.swipeManagerUsecase.Srv().GetRandomCardsFromRecentUpdates(ctx, newSwipeRecord.CardGroupID, config.Cfg.PGQueryLimit, repo.DESC, repo.DESC)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to fetch random cards")
	}

	cardAmount, err := d.swipeManagerUsecase.DetermineCardAmount(cards, d.amountOfKnownWords)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards[:cardAmount], nil
}

func (d *inWhileStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord, latestSwipeRecords []*repository.SwipeRecord) bool {
	// It needs to be certain amount of data for this mode.
	if len(latestSwipeRecords) < d.amountOfKnownWords {
		return false
	}

	// Fetch latest swipe card
	swipeRecords, err := d.swipeManagerUsecase.Srv().GetSwipeRecordsByUserAndOrder(
		ctx, newSwipeRecord.UserID, repo.DESC, 1)

	if err != nil || len(swipeRecords) == 0 {
		logger.Logger.DebugContext(ctx,
			fmt.Sprintf("amount of swipeRecords: %d or err could be nil",
				len(swipeRecords)))
		return false
	}

	// If the last swipe was a week ago or before.
	mode := time.Since(swipeRecords[0].Updated) > 168*time.Hour
	if mode {
		logger.Logger.Debug("In While Mode")
	}
	return mode
}
