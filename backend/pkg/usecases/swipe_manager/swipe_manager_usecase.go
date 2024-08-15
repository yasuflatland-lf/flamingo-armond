package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/services"
	"context"
	"fmt"
	"github.com/m-mizutani/goerr"
	"time"
)

// Enum definitions for states
const (
	DIFFICULT = "Difficult"
	GOOD      = "Good"
	EASY      = "Easy"
	DEF1      = "Def1"
	DEF2      = "Def2"
)

type SwipeManagerUsecase struct {
	swipeService     services.SwipeRecordService
	cardGroupService services.CardGroupService
	cardService      services.CardService
}

func NewSwipeManagerUsecase(
	swipeService services.SwipeRecordService,
	cardGroupService services.CardGroupService,
	cardService services.CardService) *SwipeManagerUsecase {
	return &SwipeManagerUsecase{
		swipeService:     swipeService,
		cardGroupService: cardGroupService,
		cardService:      cardService,
	}
}

// HandleSwipe Main function to execute state machine
func (s *SwipeManagerUsecase) HandleSwipe(ctx context.Context, userID int64, cardGroupID int64, order string, limit int) ([]repository.Card, error) {
	swipeRecords, err := s.swipeService.GetSwipeRecordsByUserAndOrder(ctx, userID, order, limit)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	if len(swipeRecords) == 0 {
		return s.handleNotExist(ctx, cardGroupID, order, limit)
	}

	return s.handleExist(ctx, swipeRecords)
}

func (s *SwipeManagerUsecase) handleNotExist(ctx context.Context, cardGroupID int64, order string, limit int) ([]repository.Card, error) {
	// Retrieve cards by user and card group with the specified order and limit
	cards, err := s.cardService.GetCardsByUserAndCardGroup(ctx, cardGroupID, order, limit)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards, nil
}

func (s *SwipeManagerUsecase) handleExist(ctx context.Context, swipeRecords []repository.SwipeRecord) ([]repository.Card, error) {
	var strategy StateStrategy

	switch {
	case s.isDifficult(swipeRecords):
		strategy = DifficultStateStrategy{}
	case s.isGood(swipeRecords):
		strategy = GoodStateStrategy{}
	case s.isEasy(swipeRecords):
		strategy = EasyStateStrategy{}
	case s.isDef1(swipeRecords):
		strategy = Def1StateStrategy{}
	default:
		strategy = Def2StateStrategy{}
	}

	context := &StateContext{}
	context.SetStrategy(strategy)
	return context.ExecuteStrategy(ctx, s, swipeRecords)
}

func (s *SwipeManagerUsecase) isDifficult(swipeRecords []repository.SwipeRecord) bool {
	// If the last 5 records indicate other than "known", configure difficult
	unknownCount := 0
	for i := 0; i < 5 && i < len(swipeRecords); i++ {
		if swipeRecords[i].Direction != services.KNOWN {
			unknownCount++
		}
	}
	return unknownCount == 5
}

func (s *SwipeManagerUsecase) isGood(swipeRecords []repository.SwipeRecord) bool {
	// Check if 5 out of the last 10 records are "known"
	knownCount := 0
	for i := 0; i < 10 && i < len(swipeRecords); i++ {
		if swipeRecords[i].Direction == services.KNOWN {
			knownCount++
		}
	}
	return knownCount >= 5
}

func (s *SwipeManagerUsecase) isEasy(swipeRecords []repository.SwipeRecord) bool {
	// Check if the last 5 records indicate "known"
	knownCount := 0
	for i := 0; i < 5 && i < len(swipeRecords); i++ {
		if swipeRecords[i].Direction == services.KNOWN {
			knownCount++
		}
	}
	return knownCount == 5
}

func (s *SwipeManagerUsecase) isDef1(swipeRecords []repository.SwipeRecord) bool {
	// Check if none of the conditions match and time since last swipe is significant
	// Implement the specific logic for Def1
	return time.Since(swipeRecords[0].Updated) > 24*time.Hour
}

func (s *SwipeManagerUsecase) changeState(ctx context.Context, userID int64, newState string) error {
	fmt.Printf("Changing user state to: %s\n", newState)
	// Implement logic to change user state and display cards based on state
	// ...
	return nil
}
