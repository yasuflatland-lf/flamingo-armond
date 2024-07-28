package services_test

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/testutils"
	"context"
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

	suite.Run("Normal_ListCards", func() {
		// Arrange
		createdGroup, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)
		input1 := model.NewCard{
			Front:       "Test Front 1",
			Back:        "Test Back 1",
			ReviewDate:  time.Now(),
			CardgroupID: createdGroup.ID,
		}
		input2 := model.NewCard{
			Front:       "Test Front 2",
			Back:        "Test Back 2",
			ReviewDate:  time.Now(),
			CardgroupID: createdGroup.ID,
		}
		cardService.CreateCard(ctx, input1)
		cardService.CreateCard(ctx, input2)

		// Act
		cards, err := cardService.CardsByCardGroup(ctx, createdGroup.ID)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), cards, 2)
	})

	suite.Run("Normal_CreateCard", func() {
		// Arrange
		createdGroup, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now(),
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
			ReviewDate:  time.Now(),
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
			ReviewDate:  time.Now(),
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
			ReviewDate:  time.Now(),
			CardgroupID: createdGroup.ID,
		}
		createdCard, _ := cardService.CreateCard(ctx, input)

		updateInput := model.NewCard{
			Front:       "Updated Front",
			Back:        "Updated Back",
			ReviewDate:  time.Now(),
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
			ReviewDate: time.Now(),
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
			ReviewDate:  time.Now(),
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

	suite.Run("Normal_ListCardsByCardGroup", func() {
		// Arrange
		cardGroup := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(ctx, cardGroup)
		input := model.NewCard{
			Front:       "Test Front",
			Back:        "Test Back",
			ReviewDate:  time.Now(),
			CardgroupID: createdGroup.ID,
		}
		cardService.CreateCard(ctx, input)

		// Act
		cards, err := cardService.CardsByCardGroup(ctx, createdGroup.ID)

		// Assert
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), cards, 1)
	})

	suite.Run("Error_InvalidCardGroupID_List", func() {
		// Act
		cards, err := cardService.CardsByCardGroup(ctx, -1) // Invalid ID

		// Assert
		assert.NoError(suite.T(), err)
		assert.Len(suite.T(), cards, 0)
	})
}

func TestCardTestSuite(t *testing.T) {
	suite.Run(t, new(CardTestSuite))
}
