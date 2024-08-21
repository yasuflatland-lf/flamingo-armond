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
	"time"
)

type difficultStateStrategy struct {
	swipeManagerUsecase SwipeManagerUsecase
	amountOfSwipes      int
}

type DifficultStateStrategy interface {
	SwipeStrategy
}

func NewDifficultStateStrategy(swipeManagerUsecase SwipeManagerUsecase) DifficultStateStrategy {
	return &difficultStateStrategy{
		swipeManagerUsecase: swipeManagerUsecase,
		amountOfSwipes:      config.Cfg.FLBatchDefaultAmount,
	}
}

func (d *difficultStateStrategy) Run(ctx context.Context,
	newSwipeRecord model.NewSwipeRecord) ([]*model.Card, error) {
	// Past 1 week
	fromDate := time.Now().AddDate(0, 0, -7)

	// Fetch random recent created card within a week.
	cards, err := d.swipeManagerUsecase.Srv().GetRandomRecentCards(
		ctx, fromDate, d.amountOfSwipes, repo.DESC)

	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards, nil
}

func (d *difficultStateStrategy) IsApplicable(ctx context.Context, newSwipeRecord model.NewSwipeRecord, latestSwipeRecords []*repository.SwipeRecord) bool {
	// It needs to be certain amount of data for this mode.
	if len(latestSwipeRecords) < d.amountOfSwipes {
		return false
	}

	// If the last 5 records indicate other than "known", configure difficult
	unknownCount := 0
	for i := 0; i < 5 && i < len(latestSwipeRecords); i++ {
		if latestSwipeRecords[i].Mode != services.KNOWN {
			unknownCount++
		}
	}

	mode := unknownCount == 5
	if mode {
		logger.Logger.Debug("Difficult mode")
	}
	return mode
}
