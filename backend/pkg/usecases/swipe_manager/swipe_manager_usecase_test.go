package swipe_manager

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/config"
	repo "backend/pkg/repository"
	"backend/testutils"
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type SwipeManagerTestSuite struct {
	suite.Suite
	db                 *gorm.DB
	sv                 services.Services
	cleanup            func()
	userService        services.UserService
	cardGroupService   services.CardGroupService
	cardService        services.CardService
	roleService        services.RoleService
	swipeRecordService services.SwipeRecordService
}

var migrationFilePath = "../../../db/migrations"

func (suite *SwipeManagerTestSuite) SetupSuite() {
	ctx := context.Background()
	pg, cleanup, err := testutils.SetupTestDB(ctx, "user", "password", "flamingo")
	if err != nil {
		suite.T().Fatalf("Failed to setup test database: %+v", err)
	}
	suite.cleanup = func() {
		cleanup(migrationFilePath)
	}

	if err := pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		suite.T().Fatalf("failed to run migrations: %+v", err)
	}

	suite.db = pg.GetDB()
	suite.sv = services.New(suite.db)
	suite.userService = suite.sv.(services.UserService)
	suite.cardGroupService = suite.sv.(services.CardGroupService)
	suite.cardService = suite.sv.(services.CardService)
	suite.roleService = suite.sv.(services.RoleService)
	suite.swipeRecordService = suite.sv.(services.SwipeRecordService)
}

func (suite *SwipeManagerTestSuite) TearDownSuite() {
	suite.cleanup()
}

func (suite *SwipeManagerTestSuite) TestUpdateRecords() {
	ctx := context.Background()
	usecase := &swipeManagerUsecase{
		services: suite.sv,
	}

	suite.Run("Normal_UpdateIntervalDays", func() {
		// Arrange
		card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
			suite.userService, suite.cardGroupService, suite.roleService, suite.cardService)
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
		assert.NoError(suite.T(), err)
		updatedCard, _ := suite.cardService.GetCardByID(ctx, card.ID)
		assert.Greater(suite.T(), updatedCard.IntervalDays, card.IntervalDays)
	})

	suite.Run("Normal_UpdateCardGroupUserState", func() {
		// Arrange
		card, cardGroup, user, err := testutils.CreateUserCardAndCardGroup(ctx,
			suite.userService, suite.cardGroupService, suite.roleService, suite.cardService)
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
		assert.NoError(suite.T(), err)
		cardGroupUser, _ := suite.cardGroupService.GetCardgroupUser(ctx,
			newSwipeRecord.CardGroupID, newSwipeRecord.UserID)
		assert.Equal(suite.T(), DIFFICULT, cardGroupUser.State)
	})

	suite.Run("Normal_CreateSwipeRecord", func() {
		// Arrange
		card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
			suite.userService, suite.cardGroupService, suite.roleService, suite.cardService)
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
		assert.NoError(suite.T(), err)
		swipeRecords, _ := suite.sv.GetSwipeRecordsByUserAndOrder(ctx, newSwipeRecord.UserID, repo.DESC, config.Cfg.FLBatchDefaultAmount)
		assert.NotEmpty(suite.T(), swipeRecords)
	})

	suite.Run("Normal_NewDifficultStateStrategy", func() {
		// Arrange
		card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
			suite.userService, suite.cardGroupService, suite.roleService, suite.cardService)
		assert.NoError(suite.T(), err)

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
			createdSwipeRecord, err := suite.swipeRecordService.CreateSwipeRecord(ctx,
				newSwipeRecord)
			assert.NoError(suite.T(), err)

			savedSwipeRecord = newSwipeRecord
			swipeRecords = append(swipeRecords, services.ConvertToGormSwipeRecord(*createdSwipeRecord))
		}

		// Act
		strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

		// Assert
		assert.NoError(suite.T(), err)
		assert.IsType(suite.T(), &difficultStateStrategy{}, strategy)
		assert.Equal(suite.T(), DIFFICULT, mode)
	})

	suite.Run("Normal_DefaultStateStrategy", func() {
		// Arrange
		card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
			suite.userService, suite.cardGroupService, suite.roleService, suite.cardService)
		assert.NoError(suite.T(), err)

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
		assert.NoError(suite.T(), err)
		assert.IsType(suite.T(), &defaultStateStrategy{}, strategy)
		assert.Equal(suite.T(), DEFAULT, mode)

		// Additional assertions can be added here if necessary to validate the behavior of the strategy
	})

	suite.Run("Normal_GoodStateStrategy", func() {
		// Arrange
		card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
			suite.userService, suite.cardGroupService, suite.roleService, suite.cardService)
		assert.NoError(suite.T(), err)

		// Generate 10 SwipeRecords
		savedSwipeRecord := model.NewSwipeRecord{}
		var swipeRecords []*repository.SwipeRecord
		knownCount := 0
		rng := rand.New(rand.NewSource(time.Now().UnixNano())) // Create a new random number generator

		for i := 0; i < config.Cfg.FLBatchDefaultAmount; i++ {
			mode := services.UNKNOWN // Set default mode to UNKNOWN

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
			createdSwipeRecord, err := suite.swipeRecordService.CreateSwipeRecord(ctx,
				newSwipeRecord)
			assert.NoError(suite.T(), err)

			savedSwipeRecord = newSwipeRecord
			swipeRecords = append(swipeRecords, services.ConvertToGormSwipeRecord(*createdSwipeRecord))
		}

		// Act
		if 5 <= knownCount {
			strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

			// Assert
			assert.NoError(suite.T(), err)
			assert.IsType(suite.T(), &goodStateStrategy{}, strategy)
			assert.Equal(suite.T(), GOOD, mode)
		}
	})

	suite.Run("Normal_EasyStateStrategy", func() {
		// Arrange
		card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
			suite.userService, suite.cardGroupService, suite.roleService, suite.cardService)
		assert.NoError(suite.T(), err)

		// Generate 10 SwipeRecords
		savedSwipeRecord := model.NewSwipeRecord{}
		var swipeRecords []*repository.SwipeRecord
		for i := 0; i < config.Cfg.FLBatchDefaultAmount; i++ {
			mode := services.MAYBE
			if i < 5 {
				mode = services.KNOWN // Set Mode to KNOWN for all records
			}

			newSwipeRecord := model.NewSwipeRecord{
				CardID:      card.ID,
				CardGroupID: card.CardGroupID,
				UserID:      user.ID,
				Mode:        mode,
				Created:     time.Now().UTC(),
				Updated:     time.Now().UTC(),
			}
			createdSwipeRecord, err := suite.swipeRecordService.CreateSwipeRecord(ctx,
				newSwipeRecord)
			assert.NoError(suite.T(), err)

			savedSwipeRecord = newSwipeRecord
			swipeRecords = append(swipeRecords, services.ConvertToGormSwipeRecord(*createdSwipeRecord))
		}

		// Act
		strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

		// Assert
		assert.NoError(suite.T(), err)
		assert.IsType(suite.T(), &easyStateStrategy{}, strategy)
		assert.Equal(suite.T(), EASY, mode)
	})

	suite.Run("Normal_InWhileStateStrategy", func() {
		// Arrange
		card, _, user, err := testutils.CreateUserCardAndCardGroup(ctx,
			suite.userService, suite.cardGroupService, suite.roleService, suite.cardService)
		assert.NoError(suite.T(), err)

		// Set date to two months ago
		twoMonthsAgo := time.Now().AddDate(0, -2, 0).UTC()

		// Generate sufficient SwipeRecords for the strategy to be applicable
		savedSwipeRecord := model.NewSwipeRecord{}
		var swipeRecords []*repository.SwipeRecord
		for i := 0; i < config.Cfg.FLBatchDefaultAmount; i++ {
			mode := services.KNOWN // Ensuring all records are KNOWN

			newSwipeRecord := model.NewSwipeRecord{
				CardID:      card.ID,
				CardGroupID: card.CardGroupID,
				UserID:      user.ID,
				Mode:        mode,
				Created:     twoMonthsAgo,
				Updated:     twoMonthsAgo,
			}
			createdSwipeRecord, err := suite.swipeRecordService.CreateSwipeRecord(ctx,
				newSwipeRecord)
			assert.NoError(suite.T(), err)

			savedSwipeRecord = newSwipeRecord
			swipeRecords = append(swipeRecords, services.ConvertToGormSwipeRecord(*createdSwipeRecord))
		}

		// Act
		strategy, mode, err := usecase.getStrategy(ctx, savedSwipeRecord, swipeRecords)

		// Assert
		assert.NoError(suite.T(), err)
		assert.IsType(suite.T(), &inWhileStateStrategy{}, strategy)
		assert.Equal(suite.T(), INWHILE, mode)
	})

	suite.Run("Normal_DetermineCardAmount", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, suite.userService, suite.cardGroupService, suite.roleService)

		// Create 15 dummy cards with the same cardgroup_id and store them in a slice
		var cards []model.Card
		for i := 0; i < 15; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID, // Use the same CardGroup ID for all cards
			}
			card, err := suite.cardService.CreateCard(ctx, input)
			assert.NoError(suite.T(), err)

			// Convert the created card to model.Card and append to the slice
			cards = append(cards, *card)
		}

		amountOfKnownWords := 5

		// Act
		cardAmount, err := usecase.DetermineCardAmount(cards, amountOfKnownWords)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), 5, cardAmount)
	})

	suite.Run("Error_NoCardsAvailable", func() {
		// Arrange
		cards := []model.Card{}
		amountOfKnownWords := 5

		// Act
		cardAmount, err := usecase.DetermineCardAmount(cards, amountOfKnownWords)

		// Assert
		assert.Error(suite.T(), err)
		assert.Equal(suite.T(), 0, cardAmount)
	})
}

func TestSwipeManagerTestSuite(t *testing.T) {
	suite.Run(t, new(SwipeManagerTestSuite))
}
