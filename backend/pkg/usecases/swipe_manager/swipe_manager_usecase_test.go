package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/config"
	"backend/pkg/logger"
	repo "backend/pkg/repository"
	"backend/testutils"
	"context"
	"github.com/labstack/echo/v4"
	"log"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

var db *gorm.DB
var e *echo.Echo
var sv services.Services
var userService services.UserService
var cardGroupService services.CardGroupService
var cardService services.CardService
var roleService services.RoleService
var swipeRecordService services.SwipeRecordService

var migrationFilePath = "../../../db/migrations"

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, cleanup, err := testutils.SetupTestDB(ctx, "user", "password", "flamingo")
	if err != nil {
		log.Fatalf("Failed to setup test database: %+v", err)
	}
	defer cleanup(migrationFilePath)

	if err := pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		log.Fatalf("failed to run migrations: %+v", err)
	}

	db = pg.GetDB()
	sv = services.New(db)

	userService = sv.(services.UserService)
	cardGroupService = sv.(services.CardGroupService)
	cardService = sv.(services.CardService)
	roleService = sv.(services.RoleService)
	swipeRecordService = sv.(services.SwipeRecordService)

	m.Run()
}

func TestUpdateRecords(t *testing.T) {
	t.Helper()
	t.Parallel()
	ctx := context.Background()
	usecase := &swipeManagerUsecase{
		services: sv,
	}

	testutils.RunServersTest(t, db, func(t *testing.T) {
		t.Run("Normal_UpdateIntervalDays", func(t *testing.T) {
			// Arrange
			card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
				userService, cardGroupService, roleService, cardService)
			newSwipeRecord := model.NewSwipeRecord{
				CardID:      card.ID,
				CardGroupID: card.CardGroupID,
				UserID:      user.ID,
				Mode:        services.KNOWN,
				Created:     time.Now().UTC(),
				Updated:     time.Now().UTC(),
			}

			// Act
			err = usecase.updateRecords(ctx, newSwipeRecord, GOOD)

			// Assert
			assert.NoError(t, err)
			updatedCard, _ := cardService.GetCardByID(ctx, card.ID)
			assert.Greater(t, updatedCard.IntervalDays, card.IntervalDays)
		})

		t.Run("Normal_UpdateCardGroupUserState", func(t *testing.T) {
			// Arrange
			card, cardGroup, user, err := testutils.CreateUserCardAndCardGroup(ctx,
				userService, cardGroupService, roleService, cardService)
			newSwipeRecord := model.NewSwipeRecord{
				CardID:      card.ID,
				CardGroupID: cardGroup.ID,
				UserID:      user.ID,
				Mode:        services.KNOWN,
				Created:     time.Now().UTC(),
				Updated:     time.Now().UTC(),
			}

			// Act
			err = usecase.updateRecords(ctx, newSwipeRecord, DIFFICULT)

			// Assert
			assert.NoError(t, err)
			cardGroupUser, _ := cardGroupService.GetCardgroupUser(ctx,
				newSwipeRecord.CardGroupID, newSwipeRecord.UserID)
			assert.Equal(t, DIFFICULT, cardGroupUser.State)
		})

		t.Run("Normal_CreateSwipeRecord", func(t *testing.T) {
			// Arrange
			card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
				userService, cardGroupService, roleService, cardService)
			newSwipeRecord := model.NewSwipeRecord{
				CardID:      card.ID,
				CardGroupID: card.CardGroupID,
				UserID:      user.ID,
				Mode:        services.KNOWN,
				Created:     time.Now().UTC(),
				Updated:     time.Now().UTC(),
			}

			// Act
			err = usecase.updateRecords(ctx, newSwipeRecord, EASY)

			// Assert
			assert.NoError(t, err)
			swipeRecords, _ := sv.GetSwipeRecordsByUserAndOrder(ctx, newSwipeRecord.UserID, repo.DESC, config.Cfg.FLBatchDefaultAmount)
			assert.NotEmpty(t, swipeRecords)
		})

		t.Run("Normal_DifficultStateStrategy", func(t *testing.T) {
			// Arrange
			card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
				userService, cardGroupService, roleService, cardService)
			assert.NoError(t, err)

			// Generate 10 SwipeRecords
			savedSwipeRecord := model.NewSwipeRecord{}
			var swipeRecords []*repository.SwipeRecord
			for i := 0; i < config.Cfg.FLBatchDefaultAmount; i++ {
				mode := services.MAYBE
				if i >= 5 {
					mode = services.UNKNOWN // Set Mode to UNKNOWN for records 6 to 10
				}

				newSwipeRecord := model.NewSwipeRecord{
					CardID:      card.ID,
					CardGroupID: card.CardGroupID,
					UserID:      user.ID,
					Mode:        mode,
					Created:     time.Now().UTC(),
					Updated:     time.Now().UTC(),
				}
				createdSwipeRecord, err := swipeRecordService.CreateSwipeRecord(ctx,
					newSwipeRecord)
				assert.NoError(t, err)

				savedSwipeRecord = newSwipeRecord
				swipeRecords = append(swipeRecords, services.ConvertToGormSwipeRecord(*createdSwipeRecord))
			}

			// Act
			strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

			// Assert
			assert.NoError(t, err)
			assert.IsType(t, &difficultStateStrategy{}, strategy)
			assert.Equal(t, DIFFICULT, mode)

			// Make sure if cards are returned
			cards, err := usecase.ExecuteStrategy(ctx, savedSwipeRecord, strategy)
			assert.NotEmpty(t, cards)
		})

		t.Run("Normal_DefaultStateStrategy", func(t *testing.T) {
			// Arrange
			card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
				userService, cardGroupService, roleService, cardService)
			assert.NoError(t, err)

			// Generate SwipeRecords
			savedSwipeRecord := model.NewSwipeRecord{
				CardID:      card.ID,
				CardGroupID: card.CardGroupID,
				UserID:      user.ID,
				Mode:        services.UNKNOWN,
				Created:     time.Now().UTC(),
				Updated:     time.Now().UTC(),
			}

			var swipeRecords []*repository.SwipeRecord

			// Act
			strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

			// Assert
			assert.NoError(t, err)
			assert.IsType(t, &defaultStateStrategy{}, strategy)
			assert.Equal(t, DEFAULT, mode)

			// Make sure if cards are returned
			cards, err := usecase.ExecuteStrategy(ctx, savedSwipeRecord, strategy)
			assert.NotEmpty(t, cards)
			assert.Equal(t, 1, len(cards))
		})

		t.Run("Normal_GoodStateStrategy", func(t *testing.T) {
			// Arrange
			card, cardGroup, user, err := testutils.CreateUserCardAndCardGroup(ctx,
				userService, cardGroupService, roleService, cardService)
			assert.NoError(t, err)

			// Generate 10 SwipeRecords
			savedSwipeRecord := model.NewSwipeRecord{}
			var swipeRecords []*repository.SwipeRecord
			knownCount := 0
			rng := rand.New(rand.NewSource(time.Now().UnixNano())) // Create a new random number generator

			for i := 0; i < config.Cfg.FLBatchDefaultAmount; i++ {
				mode := services.UNKNOWN // Set default mode to UNKNOWN

				input := model.NewCard{
					Front:       "Test Front" + strconv.Itoa(i),
					Back:        "Test Back" + strconv.Itoa(i),
					ReviewDate:  time.Now().UTC(),
					CardgroupID: cardGroup.ID,
				}

				_, err := cardService.CreateCard(ctx, input)

				// Randomly set 5 records to KNOWN
				if knownCount <= 5 && rng.Intn(config.Cfg.
					FLBatchDefaultAmount-knownCount) <= (5-knownCount) {
					mode = services.KNOWN
					knownCount++
				}

				newSwipeRecord := model.NewSwipeRecord{
					CardID:      card.ID,
					CardGroupID: card.CardGroupID,
					UserID:      user.ID,
					Mode:        mode,
					Created:     time.Now().UTC(),
					Updated:     time.Now().UTC(),
				}
				createdSwipeRecord, err := swipeRecordService.CreateSwipeRecord(ctx,
					newSwipeRecord)
				assert.NoError(t, err)

				savedSwipeRecord = newSwipeRecord
				swipeRecords = append(swipeRecords, services.ConvertToGormSwipeRecord(*createdSwipeRecord))
			}

			// Act
			if 5 <= knownCount {
				strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

				// Assert
				assert.NoError(t, err)
				assert.IsType(t, &goodStateStrategy{}, strategy)
				assert.Equal(t, GOOD, mode)

				// Make sure if cards are returned
				cards, err := usecase.ExecuteStrategy(ctx, savedSwipeRecord, strategy)
				assert.NotEmpty(t, cards)
				assert.Equal(t, config.Cfg.FLBatchDefaultAmount, len(cards))
			} else {
				logger.Logger.Info("Skip test due to the random data does not hit" +
					" the count.")
			}
		})

		t.Run("Normal_EasyStateStrategy", func(t *testing.T) {
			// Arrange
			card, cardGroup, user, err := testutils.CreateUserCardAndCardGroup(ctx,
				userService, cardGroupService, roleService, cardService)
			assert.NoError(t, err)

			// Generate 10 SwipeRecords
			savedSwipeRecord := model.NewSwipeRecord{}
			var swipeRecords []*repository.SwipeRecord
			for i := 0; i < config.Cfg.FLBatchDefaultAmount; i++ {
				mode := services.MAYBE
				if i < 5 {
					mode = services.KNOWN // Set Mode to KNOWN for all records
				}

				input := model.NewCard{
					Front:       "Front " + strconv.Itoa(i),
					Back:        "Back " + strconv.Itoa(i),
					ReviewDate:  time.Now().UTC(),
					CardgroupID: cardGroup.ID,
				}
				createdCard, err := cardService.CreateCard(ctx, input)
				assert.NoError(t, err)

				newSwipeRecord := model.NewSwipeRecord{
					CardID:      createdCard.ID,
					CardGroupID: card.CardGroupID,
					UserID:      user.ID,
					Mode:        mode,
					Created:     time.Now().UTC(),
					Updated:     time.Now().UTC(),
				}
				createdSwipeRecord, err := swipeRecordService.CreateSwipeRecord(ctx,
					newSwipeRecord)
				assert.NoError(t, err)

				savedSwipeRecord = newSwipeRecord
				swipeRecords = append(swipeRecords, services.ConvertToGormSwipeRecord(*createdSwipeRecord))
			}

			// Act
			strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

			// Assert
			assert.NoError(t, err)
			assert.IsType(t, &easyStateStrategy{}, strategy)
			assert.Equal(t, EASY, mode)

			// Make sure if cards are returned
			cards, err := usecase.ExecuteStrategy(ctx, savedSwipeRecord, strategy)
			assert.NotEmpty(t, cards)
			assert.Equal(t, config.Cfg.FLBatchDefaultAmount, len(cards))
		})

		t.Run("Normal_InWhileStateStrategy", func(t *testing.T) {
			// Arrange
			_, cardGroup, user, err := testutils.CreateUserCardAndCardGroup(ctx,
				userService, cardGroupService, roleService, cardService)
			assert.NoError(t, err)

			// Set date to two months ago
			twoMonthsAgo := time.Now().AddDate(0, -2, 0).UTC()

			// Generate sufficient SwipeRecords for the strategy to be applicable
			savedSwipeRecord := model.NewSwipeRecord{}
			var swipeRecords []*repository.SwipeRecord
			for i := 0; i < config.Cfg.FLBatchDefaultAmount; i++ {
				input := model.NewCard{
					Front:       "Front " + strconv.Itoa(i),
					Back:        "Back " + strconv.Itoa(i),
					ReviewDate:  time.Now().UTC(),
					CardgroupID: cardGroup.ID,
				}
				createdCard, err := cardService.CreateCard(ctx, input)
				assert.NoError(t, err)

				mode := services.KNOWN // Ensuring all records are KNOWN

				newSwipeRecord := model.NewSwipeRecord{
					CardID:      createdCard.ID,
					CardGroupID: cardGroup.ID,
					UserID:      user.ID,
					Mode:        mode,
					Created:     twoMonthsAgo,
					Updated:     twoMonthsAgo,
				}
				createdSwipeRecord, err := swipeRecordService.CreateSwipeRecord(ctx,
					newSwipeRecord)
				assert.NoError(t, err)

				savedSwipeRecord = newSwipeRecord
				swipeRecords = append(swipeRecords, services.ConvertToGormSwipeRecord(*createdSwipeRecord))
			}

			// Act
			strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

			// Assert
			assert.NoError(t, err)
			assert.IsType(t, &inWhileStateStrategy{}, strategy)
			assert.Equal(t, INWHILE, mode)

			// Make sure if cards are returned
			cards, err := usecase.ExecuteStrategy(ctx, savedSwipeRecord, strategy)
			assert.Equal(t, config.Cfg.FLBatchDefaultAmount, len(cards))
		})

		t.Run("Normal_DetermineCardAmount", func(t *testing.T) {
			// Arrange
			createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

			// Create 15 dummy cards with the same cardgroup_id and store them in a slice
			var cards []*model.Card
			for i := 0; i < 15; i++ {
				input := model.NewCard{
					Front:       "Front " + strconv.Itoa(i),
					Back:        "Back " + strconv.Itoa(i),
					ReviewDate:  time.Now().UTC(),
					CardgroupID: createdGroup.ID, // Use the same CardGroup ID for all cards
				}
				card, err := cardService.CreateCard(ctx, input)
				assert.NoError(t, err)

				// Convert the created card to model.Card and append to the slice
				cards = append(cards, card)
			}

			amountOfKnownWords := 5

			// Act
			cardAmount, err := usecase.DetermineCardAmount(cards, amountOfKnownWords)

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, 5, cardAmount)
		})

		t.Run("Error_NoCardsAvailable", func(t *testing.T) {
			// Arrange
			var cards []*model.Card
			amountOfKnownWords := 5

			// Act
			cardAmount, err := usecase.DetermineCardAmount(cards, amountOfKnownWords)

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, 0, cardAmount)
		})
	})

}
