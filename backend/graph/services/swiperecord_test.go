package services_test

import (
	"backend/graph/model"
	"backend/graph/services"
	repo "backend/pkg/repository"
	"backend/pkg/usecases/swipe_manager"
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
	userService := suite.sv.(services.UserService)
	cardGroupService := suite.sv.(services.CardGroupService)
	roleService := suite.sv.(services.RoleService)
	cardService := suite.sv.(services.CardService)
	ctx := context.Background()
	t := suite.T()
	t.Helper()

	suite.Run("Normal_CreateSwipeRecord", func() {
		createdCard, createdCardGroup, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.EASY,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}

		createdSwipeRecord, err := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		assert.NoError(t, err)
		assert.Equal(t, swipe_manager.EASY, createdSwipeRecord.Mode)
	})

	suite.Run("Error_CreateSwipeRecord", func() {

		newSwipeRecord := model.NewSwipeRecord{
			UserID: 0, // Invalid UserID
			Mode:   swipe_manager.DEFAULT,
		}

		createdSwipeRecord, err := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		assert.Error(t, err)
		assert.Nil(t, createdSwipeRecord)
	})

	suite.Run("Normal_GetSwipeRecordByID", func() {
		// Use the helper function to create the user and role
		createdCard, createdCardGroup, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.DEFAULT,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
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
		createdCard, createdCardGroup, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.EASY,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}
		createdSwipeRecord, _ := swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord)

		updateSwipeRecord := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.DIFFICULT,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}

		updatedSwipeRecord, err := swipeRecordService.UpdateSwipeRecord(ctx, createdSwipeRecord.ID, updateSwipeRecord)

		assert.NoError(t, err)
		assert.Equal(t, swipe_manager.DIFFICULT, updatedSwipeRecord.Mode)
	})

	suite.Run("Error_UpdateSwipeRecord", func() {
		// Use the helper function to create the user and role
		_, _, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		updateSwipeRecord := model.NewSwipeRecord{
			UserID: createdUser.ID,
			Mode:   swipe_manager.DIFFICULT,
		}

		updatedSwipeRecord, err := swipeRecordService.UpdateSwipeRecord(ctx, -1, updateSwipeRecord) // Invalid ID

		assert.Error(t, err)
		assert.Nil(t, updatedSwipeRecord)
	})

	suite.Run("Normal_DeleteSwipeRecord", func() {
		// Use the helper function to create the user and role
		createdCard, createdCardGroup, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.DIFFICULT,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
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
		createdCard, createdCardGroup, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord1 := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.DIFFICULT,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}
		newSwipeRecord2 := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.INWHILE,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord1)
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord2)

		swipeRecords, err := swipeRecordService.SwipeRecords(ctx)

		assert.NoError(t, err)
		assert.Len(t, swipeRecords, 2)
	})

	suite.Run("Normal_ListSwipeRecordsByUser", func() {
		// Use the helper function to create the user and role
		createdCard, createdCardGroup, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord1 := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.DIFFICULT,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}
		newSwipeRecord2 := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.EASY,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord1)
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord2)

		swipeRecords, err := swipeRecordService.SwipeRecordsByUser(ctx, createdUser.ID)

		assert.NoError(t, err)
		assert.Len(t, swipeRecords, 2)
	})

	suite.Run("Error_ListSwipeRecordsByUser", func() {
		swipeRecords, err := swipeRecordService.SwipeRecordsByUser(ctx, -1) // Invalid UserID

		assert.Error(t, err)
		assert.Nil(t, swipeRecords)
	})

	suite.Run("Normal_GetSwipeRecordsByUserAndOrder", func() {
		// Use the helper function to create the user and role
		createdCard, createdCardGroup, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		newSwipeRecord1 := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.DIFFICULT,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord1)

		newSwipeRecord2 := model.NewSwipeRecord{
			UserID:      createdUser.ID,
			CardID:      createdCard.ID,
			CardGroupID: createdCardGroup.ID,
			Mode:        swipe_manager.INWHILE,
			Created:     time.Now().UTC(),
			Updated:     time.Now().UTC(),
		}
		swipeRecordService.CreateSwipeRecord(ctx, newSwipeRecord2)

		// Fetch swipe records by user and order
		swipeRecords, err := swipeRecordService.GetSwipeRecordsByUserAndOrder(ctx, createdUser.ID, repo.DESC, 2)

		assert.NoError(t, err)
		assert.Len(t, swipeRecords, 2)
		assert.Equal(t, swipe_manager.INWHILE, swipeRecords[0].Mode) // Assuming the latest update was "right"
	})

	suite.Run("Error_GetSwipeRecordsByUserAndOrder_NoData", func() {
		// Use the helper function to create the user and role
		_, _, createdUser, err := testutils.CreateUserCardAndCardGroup(ctx, userService, cardGroupService, roleService, cardService)
		if err != nil {
			suite.T().Fatal(err)
		}

		// Fetch swipe records by user and order
		swipeRecords, err := swipeRecordService.GetSwipeRecordsByUserAndOrder(ctx, createdUser.ID, repo.DESC, 1)

		assert.NoError(t, err)
		assert.Len(t, swipeRecords, 0)
	})

	suite.Run("Error_GetSwipeRecordsByUserAndOrder", func() {
		// Attempt to fetch swipe records for an invalid user ID
		swipeRecords, err := swipeRecordService.GetSwipeRecordsByUserAndOrder(ctx, -1, repo.DESC, 2)

		assert.NoError(t, err)
		assert.Empty(t, swipeRecords)
	})

}

func TestSwipeRecordTestSuite(t *testing.T) {
	suite.Run(t, new(SwipeRecordTestSuite))
}
