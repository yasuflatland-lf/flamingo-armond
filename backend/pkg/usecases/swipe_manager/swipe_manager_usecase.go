package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/services"
	"context"
	"fmt"
	"github.com/m-mizutani/goerr"
)

// Enum definitions for states
const (
	DIFFICULT = "Difficult"
	GOOD      = "Good"
	EASY      = "Easy"
	DEF1      = "Def1"
	DEF2      = "Def2"
)

type swipeManagerUsecase struct {
	swipeService     services.SwipeRecordService
	cardGroupService services.CardGroupService
	cardService      services.CardService
}

type SwipeManagerUsecase interface {
	HandleSwipe(ctx context.Context, userID int64, cardGroupID int64, order string, limit int) ([]repository.Card, error)
	ChangeState(ctx context.Context, userID int64, newState string) error
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
func (s *swipeManagerUsecase) HandleSwipe(ctx context.Context, userID int64, cardGroupID int64, order string, limit int) ([]repository.Card, error) {
	swipeRecords, err := s.swipeService.GetSwipeRecordsByUserAndOrder(ctx, userID, order, limit)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	if len(swipeRecords) == 0 {
		return s.handleNotExist(ctx, cardGroupID, order, limit)
	}

	return s.handleExist(ctx, swipeRecords)
}

func (s *swipeManagerUsecase) handleNotExist(ctx context.Context, cardGroupID int64, order string, limit int) ([]repository.Card, error) {
	// Retrieve cards by user and card group with the specified order and limit
	cards, err := s.cardService.GetCardsByUserAndCardGroup(ctx, cardGroupID, order, limit)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards, nil
}

func (s *swipeManagerUsecase) handleExist(ctx context.Context, swipeRecords []repository.SwipeRecord) ([]repository.Card, error) {
	strategies := []SwipeStrategy{
		NewDifficultStateStrategy(s),
		NewGoodStateStrategy(s),
		NewEasyStateStrategy(s),
		NewDef1StateStrategy(s),
		NewDef2StateStrategy(s), // Default strategy, placed last
	}

	for _, strategy := range strategies {
		if strategy.IsApplicable(swipeRecords) {
			strategyExecutor := NewStrategyExecutor(strategy)
			return strategyExecutor.ExecuteStrategy(ctx, swipeRecords)
		}
	}

	return nil, nil // This should theoretically never be reached
}

func (s *swipeManagerUsecase) ChangeState(ctx context.Context, userID int64, newState string) error {
	fmt.Printf("Changing user state to: %s\n", newState)
	// Implement logic to change user state and display cards based on state
	// ...
	return nil
}
