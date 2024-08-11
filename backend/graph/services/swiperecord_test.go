package services_test

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/testutils"
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"testing"
	"time"
)

type SwipeRecordTestSuite struct {
	suite.Suite
	db      *gorm.DB
	sv      services.SwipeRecordService
	cleanup func()
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

}

func (suite *SwipeRecordTestSuite) createTestUserAndRole(ctx context.Context) (int64, error) {
	userService := suite.sv.(services.UserService)
	roleService := suite.sv.(services.RoleService)

	// Create a role
	newRole := model.NewRole{
		Name: "Test Role",
	}
	createdRole, err := roleService.CreateRole(ctx, newRole)
	if err != nil {
		return 0, fmt.Errorf("Failed to create Role: %w", err)
	}

	// Create a user
	newUser := model.NewUser{
		Name:    "Test User",
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
		RoleIds: []int64{createdRole.ID}, // Assign the new role to the user
	}
	createdUser, err := userService.CreateUser(ctx, newUser)
	if err != nil {
		return 0, fmt.Errorf("Failed to create User: %w", err)
	}

	return createdUser.ID, nil
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
		// Use the helper function to create the user and role
		userID, err := suite.createTestUserAndRole(ctx)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "left",
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
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
		// Use the helper function to create the user and role
		userID, err := suite.createTestUserAndRole(ctx)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "left",
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
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
		// Use the helper function to create the user and role
		userID, err := suite.createTestUserAndRole(ctx)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "left",
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
		}
		createdSwipeRecord, _ := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		updateSwipeRecord := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "right",
		}

		updatedSwipeRecord, err := swipeRecordService.UpdateSwipeRecord(ctx, createdSwipeRecord.ID, updateSwipeRecord)

		assert.NoError(t, err)
		assert.Equal(t, "right", updatedSwipeRecord.Direction)
	})

	suite.Run("Error_UpdateSwipeRecord", func() {
		// Use the helper function to create the user and role
		userID, err := suite.createTestUserAndRole(ctx)
		if err != nil {
			suite.T().Fatal(err)
		}

		updateSwipeRecord := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "right",
		}

		updatedSwipeRecord, err := swipeRecordService.UpdateSwipeRecord(ctx, -1, updateSwipeRecord) // Invalid ID

		assert.Error(t, err)
		assert.Nil(t, updatedSwipeRecord)
	})

	suite.Run("Normal_DeleteSwipeRecord", func() {
		// Use the helper function to create the user and role
		userID, err := suite.createTestUserAndRole(ctx)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "left",
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
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
		// Use the helper function to create the user and role
		userID, err := suite.createTestUserAndRole(ctx)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord1 := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "left",
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
		}
		newSwipeRecord2 := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "right",
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
		}
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord1)
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord2)

		swipeRecords, err := swipeRecordService.SwipeRecords(ctx)

		assert.NoError(t, err)
		assert.Len(t, swipeRecords, 2)
	})

	suite.Run("Normal_ListSwipeRecordsByUser", func() {
		// Use the helper function to create the user and role
		userID, err := suite.createTestUserAndRole(ctx)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord1 := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "left",
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
		}
		newSwipeRecord2 := model.NewSwipeRecord{
			UserID:    userID,
			Direction: "right",
			Created:   time.Now().UTC(),
			Updated:   time.Now().UTC(),
		}
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord1)
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord2)

		swipeRecords, err := swipeRecordService.SwipeRecordsByUser(ctx, userID)

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
