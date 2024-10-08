package services_test

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/graph/services"
	repo "backend/pkg/repository"
	"backend/testutils"
	"context"
	"github.com/labstack/gommon/log"
	"github.com/m-mizutani/goerr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"math/rand"
	"testing"
	"time"
)

type CardGroupTestSuite struct {
	suite.Suite
	db      *gorm.DB
	sv      services.CardGroupService
	cleanup func()
}

func (suite *CardGroupTestSuite) SetupSuite() {
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

	// Setup service
	suite.db = pg.GetDB()
	suite.sv = services.New(suite.db)
}

func (suite *CardGroupTestSuite) TearDownSuite() {
	suite.cleanup()
}

func (suite *CardGroupTestSuite) SetupSubTest() {
	t := suite.T()
	t.Helper()
	testutils.RunServersTest(t, suite.db, nil)
}

func (suite *CardGroupTestSuite) TestCardGroupService() {
	cardGroupService := suite.sv.(services.CardGroupService)
	userService := suite.sv.(services.UserService)
	roleService := suite.sv.(services.RoleService)
	ctx := context.Background()
	t := suite.T()
	t.Helper()

	suite.Run("Normal_CreateCardGroup", func() {

		input := model.NewCardGroup{Name: "Test Group"}
		createdGroup, err := cardGroupService.CreateCardGroup(context.Background(), input)

		assert.NoError(t, err)
		assert.Equal(t, "Test Group", createdGroup.Name)
	})

	suite.Run("Error_CreateCardGroup", func() {

		// Create a role
		newRole := model.NewRole{
			Name: "Test Role",
		}
		createdRole, err := roleService.CreateRole(ctx, newRole)

		// Create a user
		newUser := model.NewUser{
			Name:    "Test User",
			Created: time.Now().UTC(),
			Updated: time.Now().UTC(),
			RoleIds: []int64{createdRole.ID}, // Assign the new role to the user
		}
		createdUser, err := userService.CreateUser(ctx, newUser)

		// Create a card group
		input := model.NewCardGroup{
			Name:    "",
			Created: time.Now().UTC(),
			Updated: time.Now().UTC(),
			UserIds: []int64{createdUser.ID},
		}

		createdGroup, err := cardGroupService.CreateCardGroup(ctx, input)

		assert.Error(t, err)
		assert.Nil(t, createdGroup)
	})

	suite.Run("Normal_GetCardGroupByID", func() {

		input := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(context.Background(), input)

		fetchedGroup, err := cardGroupService.GetCardGroupByID(context.Background(), createdGroup.ID)

		assert.NoError(t, err)
		assert.Equal(t, createdGroup.ID, fetchedGroup.ID)
	})

	suite.Run("Error_GetCardGroupByID", func() {

		fetchedGroup, err := cardGroupService.GetCardGroupByID(context.Background(), -1) // Invalid ID

		assert.Error(t, err)
		assert.Nil(t, fetchedGroup)
	})

	suite.Run("Normal_UpdateCardGroup", func() {

		input := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(context.Background(), input)

		updateInput := model.NewCardGroup{Name: "Updated Group"}

		updatedGroup, err := cardGroupService.UpdateCardGroup(context.Background(), createdGroup.ID, updateInput)

		assert.NoError(t, err)
		assert.Equal(t, "Updated Group", updatedGroup.Name)
	})

	suite.Run("Error_UpdateCardGroup", func() {

		updateInput := model.NewCardGroup{Name: "Updated Group"}

		updatedGroup, err := cardGroupService.UpdateCardGroup(context.Background(), -1, updateInput) // Invalid ID

		assert.Error(t, err)
		assert.Nil(t, updatedGroup)
	})

	suite.Run("Normal_DeleteCardGroup", func() {

		input := model.NewCardGroup{Name: "Test Group"}
		createdGroup, _ := cardGroupService.CreateCardGroup(context.Background(), input)

		deleted, err := cardGroupService.DeleteCardGroup(context.Background(), createdGroup.ID)

		assert.NoError(t, err)
		assert.True(t, *deleted)
	})

	suite.Run("Error_DeleteCardGroup", func() {

		deleted, err := cardGroupService.DeleteCardGroup(context.Background(), -1) // Invalid ID

		assert.Error(t, err)
		assert.False(t, *deleted)
	})

	suite.Run("Normal_ListCardGroups", func() {

		input1 := model.NewCardGroup{Name: "Test Group 1"}
		input2 := model.NewCardGroup{Name: "Test Group 2"}
		cardGroupService.CreateCardGroup(context.Background(), input1)
		cardGroupService.CreateCardGroup(context.Background(), input2)

		groups, err := cardGroupService.CardGroups(context.Background())

		assert.NoError(t, err)
		assert.Len(t, groups, 2)
	})

	suite.Run("Normal_AddUserToCardGroup", func() {

		ctx := context.Background()
		userService := suite.sv.(services.UserService)
		cardGroupService := suite.sv.(services.CardGroupService)

		// Create a user
		newUser := model.NewUser{
			Name:    "Test User",
			Created: time.Now().UTC(),
			Updated: time.Now().UTC(),
			RoleIds: []int64{}, // Add any required roles here
		}
		createdUser, err := userService.CreateUser(ctx, newUser)
		assert.NoError(t, err)
		assert.NotNil(t, createdUser)

		// Create a card group
		newCardGroup := model.NewCardGroup{
			Name:    "Test Group",
			Created: time.Now().UTC(),
			Updated: time.Now().UTC(),
		}
		createdCardGroup, err := cardGroupService.CreateCardGroup(ctx, newCardGroup)
		assert.NoError(t, err)
		assert.NotNil(t, createdCardGroup)

		// Add user to card group
		group, err := cardGroupService.AddUserToCardGroup(ctx, createdUser.ID, createdCardGroup.ID)
		assert.NoError(t, err)
		assert.NotNil(t, group)
	})

	suite.Run("Error_AddUserToCardGroup", func() {

		userID := int64(-1)      // Invalid user ID
		cardGroupID := int64(-1) // Invalid card group ID

		group, err := cardGroupService.AddUserToCardGroup(context.Background(), userID, cardGroupID)

		assert.Error(t, err)
		assert.Nil(t, group)
	})

	suite.Run("Normal_RemoveUserFromCardGroup", func() {

		// Create a role
		newRole := model.NewRole{
			Name: "Test Role",
		}
		createdRole, err := roleService.CreateRole(ctx, newRole)
		if err != nil {
			suite.T().Fatalf("Failed at CreateRole: %+v", err)
		}

		// Create a user
		newUser := model.NewUser{
			Name:    "Test User",
			Created: time.Now().UTC(),
			Updated: time.Now().UTC(),
			RoleIds: []int64{createdRole.ID}, // Add any required roles here
		}
		createdUser, err := userService.CreateUser(ctx, newUser)
		assert.NoError(t, err)
		assert.NotNil(t, createdUser)

		// Create a card group
		input := model.NewCardGroup{
			Name:    "Test Group",
			Created: time.Now().UTC(),
			Updated: time.Now().UTC(),
			UserIds: []int64{createdUser.ID},
		}
		createdGroup, err := cardGroupService.CreateCardGroup(ctx, input)
		assert.NoError(t, err)
		assert.NotNil(t, createdGroup)

		// Add user to card group
		_, err = cardGroupService.AddUserToCardGroup(ctx, createdUser.ID, createdGroup.ID)
		assert.NoError(t, err)

		// Now remove the user from the card group
		group, err := cardGroupService.RemoveUserFromCardGroup(ctx, createdUser.ID, createdGroup.ID)
		assert.NoError(t, err)
		assert.NotNil(t, group)
	})

	suite.Run("Error_RemoveUserFromCardGroup", func() {

		userID := int64(-1)      // Invalid user ID
		cardGroupID := int64(-1) // Invalid card group ID

		group, err := cardGroupService.RemoveUserFromCardGroup(context.Background(), userID, cardGroupID)

		assert.Error(t, err)
		assert.Nil(t, group)
	})

	suite.Run("Normal_CardGroupUserState", func() {
		cardGroup, user, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)

		// Perform the UpdateCardGroupUserState operation
		newState := 3
		err := cardGroupService.UpdateCardGroupUserState(context.Background(), cardGroup.ID, user.ID, newState)
		if err != nil {
			t.Errorf("UpdateCardGroupUserState() error = %+v", goerr.Wrap(err))
		}

		// Verify the update
		var cardGroupUser repository.CardgroupUser
		if err := suite.db.Where("cardgroup_id = ? AND user_id = ?", cardGroup.ID, user.ID).First(&cardGroupUser).Error; err != nil {
			t.Fatalf("failed to retrieve updated record: %+v", goerr.Wrap(err))
		}

		if cardGroupUser.State != newState {
			t.Errorf("expected state to be %d, got %d", newState, cardGroupUser.State)
		}
	})

	suite.Run("Normal_GetLatestCardgroupUsers", func() {
		// Create an array to store the user and card group pairs
		userAndCardGroups := make([]struct {
			CardGroup model.CardGroup
			User      model.User
		}, 3)

		// Create three user and card group pairs
		for i := 0; i < len(userAndCardGroups); i++ {
			cardGroup, user, _ := testutils.CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)
			userAndCardGroups[i] = struct {
				CardGroup model.CardGroup
				User      model.User
			}{
				CardGroup: *cardGroup,
				User:      *user,
			}
		}

		// Create a new random source and generator
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))

		// Initialize the i counter
		i := 0

		// Now you can use the created user and card groups in your tests
		// Create multiple CardgroupUser records with different updated times
		for _, pair := range userAndCardGroups {
			i++ // Increment the counter for each iteration

			cardGroupUser := repository.CardgroupUser{
				CardGroupID: userAndCardGroups[0].CardGroup.ID,
				UserID:      pair.User.ID,
				State:       rng.Intn(5), // Generates a random number between 0 and 5
				Updated:     time.Now().UTC().Add(time.Duration(i) * time.Minute),
			}
			suite.db.Create(&cardGroupUser)
		}

		// Retrieve the latest CardgroupUser records with a specified limit
		limit := 3
		results, err := cardGroupService.GetLatestCardgroupUsers(ctx, userAndCardGroups[0].CardGroup.ID, limit, repo.DESC)

		// Validate the test results
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Len(t, results, limit)

		// Verify the order of updated times is correct (descending)
		for i := 0; i < len(results)-1; i++ {
			assert.True(t, results[i].Updated.After(results[i+1].Updated))
		}
	})

	suite.Run("Error_GetLatestCardgroupUsers_InvalidCardGroupID", func() {
		// Attempt to retrieve records using an invalid CardGroupID
		limit := 3
		results, err := cardGroupService.GetLatestCardgroupUsers(ctx, -1, limit, repo.DESC)

		// Check for the expected error
		assert.NoError(t, err)
		assert.Empty(t, results)
	})
}

func TestCardGroupTestSuite(t *testing.T) {
	suite.Run(t, new(CardGroupTestSuite))
}
