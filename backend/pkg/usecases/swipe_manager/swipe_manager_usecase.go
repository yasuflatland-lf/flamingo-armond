package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/config"
	"log/slog"
	"time"

	repo "backend/pkg/repository"
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

// Difficulty Define a type for the constants
type Difficulty int

// Implement the String method for the Difficulty type
func (d Difficulty) String() string {
	switch d {
	case DEFAULT:
		return "DEFAULT"
	case DIFFICULT:
		return "DIFFICULT"
	case GOOD:
		return "GOOD"
	case EASY:
		return "EASY"
	case INWHILE:
		return "INWHILE"
	default:
		return "UNKNOWN"
	}
}

type swipeManagerUsecase struct {
	services services.Services
}

type SwipeManagerUsecase interface {
	HandleSwipe(ctx context.Context, newSwipeRecord model.NewSwipeRecord) (
		[]*model.Card, error)
	Srv() services.Services
	DetermineCardAmount(cards []*model.Card, amountOfKnownWords int) (int,
		error)
}

func NewSwipeManagerUsecase(
	services services.Services) SwipeManagerUsecase {
	return &swipeManagerUsecase{
		services: services,
	}
}

func (s *swipeManagerUsecase) Srv() services.Services {
	return s.services
}

// HandleSwipe Main function to execute state machine
func (s *swipeManagerUsecase) HandleSwipe(ctx context.Context,
	newSwipeRecord model.NewSwipeRecord) ([]*model.Card, error) {

	// Fetch latest swipe records
	latestSwipeRecords, err := s.Srv().GetSwipeRecordsByUserAndOrder(ctx, newSwipeRecord.UserID, repo.DESC, config.Cfg.FLBatchDefaultAmount)
	if err != nil {
		return nil, goerr.Wrap(err)
	}

	// Start a transaction
	tx, err := s.Srv().BeginTx(ctx)
	if err != nil {
		tx.Rollback()
		return nil, goerr.Wrap(err, "failed to begin transaction")
	}

	// Get matched strategy
	strategy, mode, err := s.getStrategy(ctx, newSwipeRecord,
		latestSwipeRecords)
	if err != nil {
		tx.Rollback()
		return nil, goerr.Wrap(err)
	}

	// Exec Strategy
	cards, err := s.ExecuteStrategy(ctx, newSwipeRecord, strategy)
	if err != nil {
		tx.Rollback()
		return nil, goerr.Wrap(err, slog.Int("Failed to execute strategy Mode:", mode))
	}

	// Update the mode of cardgroup_user
	err = s.updateRecords(ctx, newSwipeRecord, mode)
	if err != nil {
		tx.Rollback()
		return nil, goerr.Wrap(err, "failed to update records")
	}

	// Commit the transaction
	tx.Commit()

	return cards, nil
}

// Update records
func (s *swipeManagerUsecase) updateRecords(
	ctx context.Context,
	newSwipeRecord model.NewSwipeRecord,
	mode int) error {

	// Fetch the Card by ID
	card, err := s.Srv().GetCardByID(ctx, newSwipeRecord.CardID)
	if err != nil {
		return goerr.Wrap(err, "failed to fetch card by ID")
	}

	// Update the interval days using the logic
	intervalLogic := NewIntervalLogic()
	updatedIntervalDays, updatedreviewDate := intervalLogic.UpdateInterval(
		card.IntervalDays,
		card.ReviewDate,
		mode)

	// Prepare the updated card data
	updatedCard := model.NewCard{
		Front:        card.Front,
		Back:         card.Back,
		ReviewDate:   updatedreviewDate,
		IntervalDays: &updatedIntervalDays,
		CardgroupID:  card.CardGroupID,
		Created:      card.Created,
		Updated:      time.Now().UTC(),
	}

	// Update the Card using UpdateCard service
	_, err = s.Srv().UpdateCard(ctx, card.ID, updatedCard)
	if err != nil {
		return goerr.Wrap(err, "failed to update card")
	}

	// Update the mode of cardgroup_user
	err = s.Srv().UpdateCardGroupUserState(ctx, newSwipeRecord.CardGroupID, newSwipeRecord.UserID, mode)
	if err != nil {
		return goerr.Wrap(err, "failed to update card group user state")
	}

	// Create a new swipe record
	_, err = s.Srv().CreateSwipeRecord(ctx, newSwipeRecord)
	if err != nil {
		return goerr.Wrap(err, "failed to update swipe record")
	}

	return nil
}

// Match Strategy
func (s *swipeManagerUsecase) getStrategy(
	ctx context.Context,
	newSwipeRecord model.NewSwipeRecord,
	latestSwipeRecords []*repository.SwipeRecord) (SwipeStrategy, int, error) {
	strategies := []struct {
		strategy SwipeStrategy
		mode     int
	}{
		// **********************************************************
		// Be careful to change the order of the strategy.
		// It affects how the strategy works.
		// Please do make sure the behavior by writing tests.
		// **********************************************************
		{NewInWhileStateStrategy(s), INWHILE},
		{NewDifficultStateStrategy(s), DIFFICULT},
		{NewEasyStateStrategy(s), EASY},
		{NewGoodStateStrategy(s), GOOD},
		// Default strategy, must be placed last
		{NewDefaultStateStrategy(s), DEFAULT},
	}

	for _, item := range strategies {
		if item.strategy.IsApplicable(ctx, newSwipeRecord, latestSwipeRecords) {
			return item.strategy, item.mode, nil
		}
	}
	return nil, services.UNDEFINED, goerr.New("Strategy unmatched")
}

func (s *swipeManagerUsecase) ExecuteStrategy(ctx context.Context,
	newSwipeRecord model.NewSwipeRecord, strategy SwipeStrategy) (
	[]*model.Card, error) {
	// Change State
	cards, err := strategy.Run(ctx, newSwipeRecord)

	if err != nil {
		return nil, goerr.Wrap(err)
	}

	return cards, nil
}

func (s *swipeManagerUsecase) DetermineCardAmount(
	cards []*model.Card,
	amountOfKnownWords int) (int, error) {
	cardAmount := amountOfKnownWords
	if len(cards) <= amountOfKnownWords {
		cardAmount = len(cards)
		if cardAmount < 0 {
			cardAmount = 0
		}
	}

	return cardAmount, nil
}
