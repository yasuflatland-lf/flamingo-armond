package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/services"
	"context"
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
	HandleSwipe(ctx context.Context, latestSwipeRecord repository.SwipeRecord, order string, limit int) ([]repository.Card, error)
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
func (s *swipeManagerUsecase) HandleSwipe(ctx context.Context, latestSwipeRecord repository.SwipeRecord, order string, limit int) ([]repository.Card, error) {
	swipeRecords, err := s.swipeService.GetSwipeRecordsByUserAndOrder(ctx, latestSwipeRecord.UserID, order, limit)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	if len(swipeRecords) == 0 {
		return s.handleNotExist(ctx, latestSwipeRecord, order, limit)
	}

	return s.handleExist(ctx, latestSwipeRecord)
}

func (s *swipeManagerUsecase) handleNotExist(ctx context.Context, latestSwipeRecord repository.SwipeRecord, order string, limit int) ([]repository.Card, error) {
	// Retrieve cards by user and card group with the specified order and limit
	cards, err := s.cardService.GetCardsByUserAndCardGroup(ctx, latestSwipeRecord.CardGroupID, order, limit)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards, nil
}

func (s *swipeManagerUsecase) handleExist(ctx context.Context, latestSwipeRecord repository.SwipeRecord) ([]repository.Card, error) {
	strategies := []SwipeStrategy{
		NewDifficultStateStrategy(s),
		NewGoodStateStrategy(s),
		NewEasyStateStrategy(s),
		NewInWhileStateStrategy(s),
		NewDefaultStateStrategy(s), // Default strategy, placed last
	}

	for _, strategy := range strategies {
		if strategy.IsApplicable(ctx, latestSwipeRecord) {
			strategyExecutor := NewStrategyExecutor(strategy)
			return strategyExecutor.ExecuteStrategy(ctx, latestSwipeRecord)
		}
	}

	return nil, nil // This should theoretically never be reached
}

func (s *swipeManagerUsecase) ChangeState(ctx context.Context, cardGroupID int64, userID int64, newState int) error {
	// Call the UpdateCardGroupUserState method from CardGroupService
	err := s.cardGroupService.UpdateCardGroupUserState(ctx, cardGroupID, userID, newState)
	if err != nil {
		return goerr.Wrap(err, "failed to update card group user state")
	}

	return nil
}
