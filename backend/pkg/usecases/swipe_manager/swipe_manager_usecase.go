package swipe_manager

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/config"
	"backend/pkg/repository"
	"context"
	"fmt"
	"github.com/m-mizutani/goerr"
)

// Enum definitions for states
const (
	DEFAULT   = 0
	DIFFICULT = 1
	GOOD      = 2
	EASY      = 3
	INWHILE   = 4
)

type swipeManagerUsecase struct {
	swipeService     services.SwipeRecordService
	cardGroupService services.CardGroupService
	cardService      services.CardService
}

type SwipeManagerUsecase interface {
	HandleSwipe(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error)
	ChangeState(ctx context.Context, cardGroupID int64, userID int64, newState int) error
}

func NewSwipeManagerUsecase(
	swipeService services.SwipeRecordService,
	cardGroupService services.CardGroupService,
	cardService services.CardService) SwipeManagerUsecase {
	return &swipeManagerUsecase{
		swipeService:     swipeService,
		cardGroupService: cardGroupService,
		cardService:      cardService,
	}
}

// HandleSwipe Main function to execute state machine
func (s *swipeManagerUsecase) HandleSwipe(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	latestSwipeRecord, err := s.swipeService.CreateSwipeRecord(ctx, newSwipeRecord)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	latestSwipeRecords, err := s.swipeService.GetSwipeRecordsByUserAndOrder(ctx, latestSwipeRecord.UserID, repository.DESC, config.Cfg.FLBatchAmount)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	if len(latestSwipeRecords) <= 1 {
		return s.handleNotExist(ctx, newSwipeRecord)
	}

	return s.handleExist(ctx, newSwipeRecord)
}

func (s *swipeManagerUsecase) handleNotExist(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	// Retrieve cards by user and card group with the specified order and limit
	cards, err := s.cardService.GetCardsByUserAndCardGroup(ctx, newSwipeRecord.CardGroupID, repository.DESC, config.Cfg.FLBatchAmount)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return services.ConvertToCards(cards), nil
}

func (s *swipeManagerUsecase) handleExist(ctx context.Context, newSwipeRecord model.NewSwipeRecord) ([]model.Card, error) {
	strategies := []SwipeStrategy{
		NewDifficultStateStrategy(s),
		NewGoodStateStrategy(s),
		NewEasyStateStrategy(s),
		NewInWhileStateStrategy(s),
		NewDefaultStateStrategy(s), // Default strategy, placed last
	}

	for _, strategy := range strategies {
		if strategy.IsApplicable(ctx, newSwipeRecord) {
			return s.ExecuteStrategy(ctx, newSwipeRecord, strategy)
		}
	}

	return nil, goerr.Wrap(fmt.Errorf("fatal error : Strategy selection")) // This should theoretically never be reached
}

func (s *swipeManagerUsecase) ExecuteStrategy(ctx context.Context, newSwipeRecord model.NewSwipeRecord, strategy SwipeStrategy) ([]model.Card, error) {
	// Change State
	cards, err := strategy.Run(ctx, newSwipeRecord)

	if err != nil {
		return nil, goerr.Wrap(err)
	}

	// Swipe
	// users_card
	// update card
	// return next batch cards
	return cards, nil
}

func (s *swipeManagerUsecase) ChangeState(ctx context.Context, cardGroupID int64, userID int64, newState int) error {

	err := s.cardGroupService.UpdateCardGroupUserState(ctx, cardGroupID, userID, newState)
	if err != nil {
		return goerr.Wrap(err, "failed to update card group user state")
	}

	return nil
}
