package services_test

import (
	"backend/graph/model"
	"backend/graph/services"
	"backend/testutils"
	"context"
	"github.com/labstack/gommon/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"testing"
	"time"
)

type UserTestSuite struct {
	suite.Suite
	db      *gorm.DB
	sv      services.UserService
	roleSvc services.RoleService
	cleanup func()
}

func (suite *UserTestSuite) SetupSuite() {
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

func (suite *UserTestSuite) TearDownSuite() {
	suite.cleanup()
}

func (suite *UserTestSuite) SetupSubTest() {
	t := suite.T()
	t.Helper()
	testutils.RunServersTest(t, suite.db, nil)
}

func (suite *UserTestSuite) TestUserService() {
	userService := suite.sv.(services.UserService)
	roleService := suite.sv.(services.RoleService)
	ctx := context.Background()
	t := suite.T()
	t.Helper()

	suite.Run("Normal_CreateUser", func() {

		// Create a role
		newRole := model.NewRole{
			Name: "Test Role",
		}
		createdRole, err := roleService.CreateRole(ctx, newRole)
		assert.NoError(t, err)
		assert.NotNil(t, createdRole)

		input := model.NewUser{
			Name:     "Test User",
			Email:    testutils.GetRandomEmail(8),
			GoogleID: testutils.GenerateUUIDv7(),
			Created:  time.Now().UTC(),
			Updated:  time.Now().UTC(),
			RoleIds:  []int64{createdRole.ID},
		}

		createdUser, err := userService.CreateUser(ctx, input)
		assert.NoError(t, err)
		assert.Equal(t, "Test User", createdUser.Name)
	})

	suite.Run("Error_CreateUser", func() {

		// Create a role
		newRole := model.NewRole{
			Name: "Test Role",
		}
		createdRole, err := roleService.CreateRole(ctx, newRole)
		assert.NoError(t, err)
		assert.NotNil(t, createdRole)

		input := model.NewUser{
			Name:     "", // Invalid input
			Email:    testutils.GetRandomEmail(8),
			GoogleID: testutils.GenerateUUIDv7(),
			Created:  time.Now().UTC(),
			Updated:  time.Now().UTC(),
			RoleIds:  []int64{createdRole.ID},
		}

		createdUser, err := userService.CreateUser(ctx, input)
		assert.Error(t, err)
		assert.Nil(t, createdUser)
	})

	suite.Run("Normal_GetUserByID", func() {

		// Create a role
		newRole := model.NewRole{
			Name: "Test Role",
		}
		createdRole, err := roleService.CreateRole(ctx, newRole)
		assert.NoError(t, err)
		assert.NotNil(t, createdRole)

		input := model.NewUser{
			Name:     "Test User",
			Email:    testutils.GetRandomEmail(8),
			GoogleID: testutils.GenerateUUIDv7(),
			Created:  time.Now().UTC(),
			Updated:  time.Now().UTC(),
			RoleIds:  []int64{createdRole.ID},
		}
		createdUser, _ := userService.CreateUser(ctx, input)

		fetchedUser, err := userService.GetUserByID(ctx, createdUser.ID)
		assert.NoError(t, err)
		assert.Equal(t, createdUser.ID, fetchedUser.ID)
	})

	suite.Run("Error_GetUserByID", func() {

		fetchedUser, err := userService.GetUserByID(ctx, -1) // Invalid ID
		assert.Error(t, err)
		assert.Nil(t, fetchedUser)
	})

	suite.Run("Normal_UpdateUser", func() {
		// Create a role
		newRole := model.NewRole{
			Name: "Test Role",
		}
		createdRole, err := roleService.CreateRole(ctx, newRole)
		assert.NoError(t, err)
		assert.NotNil(t, createdRole)

		input := model.NewUser{
			Name:     "Test User",
			Email:    testutils.GetRandomEmail(8),
			GoogleID: testutils.GenerateUUIDv7(),
			Created:  time.Now().UTC(),
			Updated:  time.Now().UTC(),
			RoleIds:  []int64{createdRole.ID},
		}
		createdUser, _ := userService.CreateUser(ctx, input)

		updateInput := model.NewUser{Name: "Updated User"}

		updatedUser, err := userService.UpdateUser(ctx, createdUser.ID, updateInput)
		assert.NoError(t, err)
		assert.Equal(t, "Updated User", updatedUser.Name)
	})

	suite.Run("Error_UpdateUser", func() {

		updateInput := model.NewUser{Name: "Updated User",
			Email:    testutils.GetRandomEmail(8),
			GoogleID: testutils.GenerateUUIDv7()}

		updatedUser, err := userService.UpdateUser(ctx, -1, updateInput) // Invalid ID
		assert.Error(t, err)
		assert.Nil(t, updatedUser)
	})

	suite.Run("Normal_DeleteUser", func() {

		// Create a role
		newRole := model.NewRole{
			Name: "Test Role",
		}
		createdRole, err := roleService.CreateRole(ctx, newRole)
		assert.NoError(t, err)
		assert.NotNil(t, createdRole)

		input := model.NewUser{
			Name:     "Test User",
			Email:    testutils.GetRandomEmail(8),
			GoogleID: testutils.GenerateUUIDv7(),
			Created:  time.Now().UTC(),
			Updated:  time.Now().UTC(),
			RoleIds:  []int64{createdRole.ID},
		}
		createdUser, _ := userService.CreateUser(ctx, input)

		deleted, err := userService.DeleteUser(ctx, createdUser.ID)
		assert.NoError(t, err)
		assert.True(t, *deleted)
	})

	suite.Run("Error_DeleteUser", func() {

		deleted, err := userService.DeleteUser(ctx, -1) // Invalid ID
		assert.NoError(t, err)
		assert.True(t, *deleted)
	})

	suite.Run("Normal_ListUsers", func() {

		// Create a role
		newRole := model.NewRole{
			Name: "Test Role",
		}
		createdRole, err := roleService.CreateRole(ctx, newRole)
		assert.NoError(t, err)
		assert.NotNil(t, createdRole)

		input1 := model.NewUser{Name: "Test User 1",
			Email:    testutils.GetRandomEmail(8),
			GoogleID: testutils.GenerateUUIDv7(),
			RoleIds:  []int64{createdRole.ID}}
		input2 := model.NewUser{Name: "Test User 2",
			Email:    testutils.GetRandomEmail(8),
			GoogleID: testutils.GenerateUUIDv7(),
			RoleIds:  []int64{createdRole.ID}}
		userService.CreateUser(ctx, input1)
		userService.CreateUser(ctx, input2)

		users, err := userService.Users(ctx)
		assert.NoError(t, err)
		assert.Len(t, users, 2)
	})
}

func TestUserTestSuite(t *testing.T) {
	suite.Run(t, new(UserTestSuite))
}
