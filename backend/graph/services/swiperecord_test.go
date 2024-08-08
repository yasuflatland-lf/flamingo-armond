package services_test

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/testutils"
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"testing"
	"time"
)

type SwipeRecordTestSuite struct {
	suite.Suite
	db        *gorm.DB
	sv        services.SwipeRecordService
	userID    int64
	cardGroup *model.CardGroup
	cleanup   func()
}

func (suite *SwipeRecordTestSuite) SetupSuite() {
	// Setup context
	ctx := context.Background()

	// Set up the test database
	pg, cleanup, err := testutils.SetupTestDB(ctx, "user", "password", "dbname")
	if err != nil {
		suite.T().Fatalf("Failed to setup test database: %+v", err)
	}
	suite.cleanup = func() {
		cleanup(migrationFilePath)
	}

	// Run migrations
	if err := pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		suite.T().Fatalf("Failed to run migrations: %+v", err)
	}

	// Setup service
	suite.db = pg.GetDB()
	suite.sv = services.New(suite.db)

	// Create a user and card group
	userService := suite.sv.(services.UserService)
	cardGroupService := suite.sv.(services.CardGroupService)
	roleService := suite.sv.(services.RoleService)

	createdGroup, err := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)
	if err != nil {
		suite.T().Fatalf("Failed to create user and card group: %+v", err)
	}
	suite.userID = createdGroup.Users.Nodes[0].ID
	suite.cardGroup = createdGroup
}

func (suite *SwipeRecordTestSuite) TearDownSuite() {
	suite.cleanup()
}

func (suite *SwipeRecordTestSuite) SetupSubTest() {
	t := suite.T()
	t.Helper()
	testutils.RunServersTest(t, suite.db, nil)
}

func (suite *SwipeRecordTestSuite) TestSwipeRecordService() {
	swipeRecordService := suite.sv.(services.SwipeRecordService)
	ctx := context.Background()
	t := suite.T()
	t.Helper()

	suite.Run("Normal_CreateSwipeRecord", func() {
		newSwipeRecord := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "left",
			Created:   time.Now(),
			Updated:   time.Now(),
		}

		createdSwipeRecord, err := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		assert.NoError(t, err)
		assert.Equal(t, "left", createdSwipeRecord.Direction)
	})

	suite.Run("Error_CreateSwipeRecord", func() {
		newSwipeRecord := model.NewSwipeRecord{
			UserID:    0, // Invalid UserID
			Direction: "",
		}

		createdSwipeRecord, err := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		assert.Error(t, err)
		assert.Nil(t, createdSwipeRecord)
	})

	suite.Run("Normal_GetSwipeRecordByID", func() {
		newSwipeRecord := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "left",
			Created:   time.Now(),
			Updated:   time.Now(),
		}
		createdSwipeRecord, _ := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		fetchedSwipeRecord, err := swipeRecordService.GetSwipeRecordByID(ctx, createdSwipeRecord.ID)

		assert.NoError(t, err)
		assert.Equal(t, createdSwipeRecord.ID, fetchedSwipeRecord.ID)
	})

	suite.Run("Error_GetSwipeRecordByID", func() {
		fetchedSwipeRecord, err := swipeRecordService.GetSwipeRecordByID(ctx, -1) // Invalid ID

		assert.Error(t, err)
		assert.Nil(t, fetchedSwipeRecord)
	})

	suite.Run("Normal_UpdateSwipeRecord", func() {
		newSwipeRecord := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "left",
			Created:   time.Now(),
			Updated:   time.Now(),
		}
		createdSwipeRecord, _ := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		updateSwipeRecord := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "right",
		}

		updatedSwipeRecord, err := swipeRecordService.UpdateSwipeRecord(ctx, createdSwipeRecord.ID, updateSwipeRecord)

		assert.NoError(t, err)
		assert.Equal(t, "right", updatedSwipeRecord.Direction)
	})

	suite.Run("Error_UpdateSwipeRecord", func() {
		updateSwipeRecord := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "right",
		}

		updatedSwipeRecord, err := swipeRecordService.UpdateSwipeRecord(ctx, -1, updateSwipeRecord) // Invalid ID

		assert.Error(t, err)
		assert.Nil(t, updatedSwipeRecord)
	})

	suite.Run("Normal_DeleteSwipeRecord", func() {
		newSwipeRecord := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "left",
			Created:   time.Now(),
			Updated:   time.Now(),
		}
		createdSwipeRecord, _ := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		deleted, err := swipeRecordService.DeleteSwipeRecord(ctx, createdSwipeRecord.ID)

		assert.NoError(t, err)
		assert.True(t, *deleted)
	})

	suite.Run("Error_DeleteSwipeRecord", func() {
		deleted, err := swipeRecordService.DeleteSwipeRecord(ctx, -1) // Invalid ID

		assert.Error(t, err)
		assert.False(t, *deleted)
	})

	suite.Run("Normal_ListSwipeRecords", func() {
		newSwipeRecord1 := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "left",
			Created:   time.Now(),
			Updated:   time.Now(),
		}
		newSwipeRecord2 := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "right",
			Created:   time.Now(),
			Updated:   time.Now(),
		}
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord1)
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord2)

		swipeRecords, err := swipeRecordService.SwipeRecords(ctx)

		assert.NoError(t, err)
		assert.Len(t, swipeRecords, 2)
	})

	suite.Run("Normal_ListSwipeRecordsByUser", func() {
		newSwipeRecord1 := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "left",
			Created:   time.Now(),
			Updated:   time.Now(),
		}
		newSwipeRecord2 := model.NewSwipeRecord{
			UserID:    suite.userID,
			Direction: "right",
			Created:   time.Now(),
			Updated:   time.Now(),
		}
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord1)
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord2)

		swipeRecords, err := swipeRecordService.SwipeRecordsByUser(ctx, suite.userID)

		assert.NoError(t, err)
		assert.Len(t, swipeRecords, 2)
	})

	suite.Run("Error_ListSwipeRecordsByUser", func() {
		swipeRecords, err := swipeRecordService.SwipeRecordsByUser(ctx, -1) // Invalid UserID

		assert.Error(t, err)
		assert.Nil(t, swipeRecords)
	})
}

func TestSwipeRecordTestSuite(t *testing.T) {
	suite.Run(t, new(SwipeRecordTestSuite))
}
