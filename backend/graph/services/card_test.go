package services_test

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/testutils"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/labstack/gommon/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type CardTestSuite struct {
	suite.Suite
	db      *gorm.DB
	sv      services.Services
	cleanup func()
}

var migrationFilePath = "../../db/migrations"

func (suite *CardTestSuite) SetupSuite() {
	// Setup context
	ctx := context.Background()

	// Set up the test database
	pg, cleanup, err := testutils.SetupTestDB(ctx, "user", "password", "flamingo")
	if err != nil {
		log.Fatalf("Failed to setup test database: %+v", err)
	}
	suite.cleanup = func() {
		cleanup(migrationFilePath)
	}

	// Run migrations
	if err := pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		log.Fatalf("failed to run migrations: %+v", err)
	}

	// Setup Echo server
	suite.db = pg.GetDB()
	suite.sv = services.New(suite.db)
}

func (suite *CardTestSuite) TearDownSuite() {
	suite.cleanup()
}

func (suite *CardTestSuite) SetupSubTest() {
	t := suite.T()
	t.Helper()
	testutils.RunServersTest(t, suite.db, nil)
}

func (suite *CardTestSuite) TestCardService() {
	cardService := suite.sv.(services.CardService)
	cardGroupService := suite.sv.(services.CardGroupService)
	userService := suite.sv.(services.UserService)
	roleService := suite.sv.(services.RoleService)
	ctx := context.Background()
	t := suite.T()
	t.Helper()

	suite.Run("Normal_CreateCard", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}

		// Act
		createdCard, err := cardService.CreateCard(ctx, input)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), "Test Front", createdCard.Front)
		assert.Equal(suite.T(), "Test Back", createdCard.Back)
	})

	suite.Run("Error_InvalidCardGroupID", func() {
		// Arrange
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: -1, // Invalid ID
		}

		// Act
		createdCard, err := cardService.CreateCard(ctx, input)

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), createdCard)
	})

	suite.Run("Normal_GetCardByID", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}
		createdCard, _ := cardService.CreateCard(ctx, input)

		// Act
		fetchedCard, err := cardService.GetCardByID(ctx, createdCard.ID)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), createdCard.ID, fetchedCard.ID)
	})

	suite.Run("Error_CardNotFound", func() {
		// Act
		fetchedCard, err := cardService.GetCardByID(ctx, -1) // Invalid ID

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), fetchedCard)
	})

	suite.Run("Normal_UpdateCard", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}
		createdCard, _ := cardService.CreateCard(ctx, input)

		updateInput := model.NewCard{
			Front:       "Updated Front",
			Back:        "Updated Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}

		// Act
		updatedCard, err := cardService.UpdateCard(ctx, createdCard.ID, updateInput)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Equal(suite.T(), "Updated Front", updatedCard.Front)
		assert.Equal(suite.T(), "Updated Back", updatedCard.Back)
	})

	suite.Run("Error_CardNotFound_Update", func() {
		// Arrange
		updateInput := model.NewCard{
			Front:      "Updated Front",
			Back:       "Updated Back",
			ReviewDate: time.Now().UTC(),
		}

		// Act
		updatedCard, err := cardService.UpdateCard(ctx, -1, updateInput) // Invalid ID

		// Assert
		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), updatedCard)
	})

	suite.Run("Normal_DeleteCard", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now().UTC(),
			CardgroupID: createdGroup.ID,
		}
		createdCard, _ := cardService.CreateCard(ctx, input)

		// Act
		deleted, err := cardService.DeleteCard(ctx, createdCard.ID)

		// Assert
		assert.NoError(suite.T(), err)
		assert.True(suite.T(), *deleted)
	})

	suite.Run("Error_CardNotFound_Delete", func() {
		// Act
		deleted, err := cardService.DeleteCard(ctx, -1) // Invalid ID

		// Assert
		assert.Error(suite.T(), err)
		assert.False(suite.T(), *deleted)
	})

	suite.Run("Normal_FetchAllCardsByCardGroup", func() {
		// Arrange
		createdGroup, _, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Create 200 dummy cards
		for i := 0; i < 200; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			_, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)
		}

		// Act
		var first = 8
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, &first)

		// Assert
		assert.NoError(t, err)
		assert.Len(t, allCards, 200) // Ensure all 200 cards are retrieved
	})

	suite.Run("Normal_AddNewCards_Matching5Of8", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create existing 5 cards
		existingCards := []model.Card{}
		for i := 0; i < 5; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			card, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)
			existingCards = append(existingCards, *card)
		}

		// Create 8 target cards (5 matching existing, 3 new)
		targetCards := []model.Card{}
		for i := 0; i < 8; i++ {
			targetCard := model.Card{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardGroupID: createdGroup.ID,
			}
			targetCards = append(targetCards, targetCard)
		}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 8) // Ensure all 8 cards are present, 3 added
	})

	suite.Run("Normal_AddNewCards_Matching3Of7", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create existing 5 cards
		existingCards := []model.Card{}
		for i := 0; i < 5; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			card, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)
			existingCards = append(existingCards, *card)
		}

		// Create 7 target cards (3 matching existing, 4 new)
		targetCards := []model.Card{}
		for i := 2; i < 9; i++ {
			targetCard := model.Card{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardGroupID: createdGroup.ID,
			}
			targetCards = append(targetCards, targetCard)
		}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 9) // 4 new cards added to the 5 existing ones
	})

	suite.Run("Normal_AddNewCards_NoTargetCards", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create existing 5 cards
		existingCards := []model.Card{}
		for i := 0; i < 5; i++ {
			input := model.NewCard{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardgroupID: createdGroup.ID,
			}
			card, err := cardService.CreateCard(ctx, input)
			assert.NoError(t, err)
			existingCards = append(existingCards, *card)
		}

		// No target cards
		targetCards := []model.Card{}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 5) // No new cards added
	})

	suite.Run("Normal_AddNewCards_EmptyExistingWithTargetCards", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create 5 target cards
		targetCards := []model.Card{}
		for i := 0; i < 5; i++ {
			targetCard := model.Card{
				Front:       "Front " + strconv.Itoa(i),
				Back:        "Back " + strconv.Itoa(i),
				ReviewDate:  time.Now().UTC(),
				CardGroupID: createdGroup.ID,
			}
			targetCards = append(targetCards, targetCard)
		}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 5) // All 5 target cards added
	})

	suite.Run("Normal_AddNewCards_BothEmpty", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// No target cards
		targetCards := []model.Card{}

		// Act
		_, err := cardService.AddNewCards(ctx, targetCards, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		allCards, err := cardService.FetchAllCardsByCardGroup(ctx, createdGroup.ID, nil)
		assert.NoError(t, err)
		assert.Len(t, allCards, 0) // No cards present
	})

	suite.Run("Normal_AddNewCards_UpdateOnBackDifference", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)

		// Create an existing card with specific Front and Back
		intervalDays := 1 // Set an appropriate default value
		input := model.NewCard{
			Front:        "Test Front",
			Back:         "Test Back",
			ReviewDate:   time.Now().UTC(),
			IntervalDays: &intervalDays, // Set IntervalDays here
			CardgroupID:  createdGroup.ID,
		}
		createdCard, err := cardService.CreateCard(ctx, input)
		assert.NoError(t, err)

		// Create a target card with the same Front but slightly different Back
		targetCard := model.Card{
			Front:        "Test Front",
			Back:         "Test BackX", // 1 character difference
			ReviewDate:   time.Now().UTC(),
			IntervalDays: intervalDays, // Similarly set IntervalDays
			CardGroupID:  createdGroup.ID,
		}

		// Act
		_, err = cardService.AddNewCards(ctx, []model.Card{targetCard}, createdGroup.ID)

		// Assert
		assert.NoError(t, err)
		updatedCard, err := cardService.GetCardByID(ctx, createdCard.ID)
		assert.NoError(t, err)
		assert.Equal(t, "Test BackX", updatedCard.Back) // Ensure the Back was updated
		assert.Equal(t, createdCard.ID, updatedCard.ID) // Ensure the same card ID is retained
	})

}

func TestCardTestSuite(t *testing.T) {
	suite.Run(t, new(CardTestSuite))
}
