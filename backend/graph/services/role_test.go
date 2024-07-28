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

type RoleTestSuite struct {
	suite.Suite
	db      *gorm.DB
	sv      services.RoleService
	cleanup func()
}

func (suite *RoleTestSuite) SetupSuite() {
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

func (suite *RoleTestSuite) TearDownSuite() {
	suite.cleanup()
}

func (suite *RoleTestSuite) SetupSubTest() {
	t := suite.T()
	t.Helper()
	testutils.RunServersTest(t, suite.db, nil)
}

func (suite *RoleTestSuite) TestRoleService() {
	userService := suite.sv.(services.UserService)
	roleService := suite.sv.(services.RoleService)
	ctx := context.Background()
	t := suite.T()
	t.Helper()

	suite.Run("Normal_CreateRole", func() {

		newRole := model.NewRole{
			Name: "Test Role",
		}

		createdRole, err := roleService.CreateRole(ctx, newRole)

		assert.NoError(t, err)
		assert.Equal(t, "Test Role", createdRole.Name)
	})

	suite.Run("Error_CreateRole", func() {

		newRole := model.NewRole{
			Name: "", // Invalid name
		}

		createdRole, err := roleService.CreateRole(ctx, newRole)

		assert.Error(t, err)
		assert.Nil(t, createdRole)
	})

	suite.Run("Normal_GetRoleByID", func() {

		newRole := model.NewRole{Name: "Test Role"}
		createdRole, _ := roleService.CreateRole(ctx, newRole)

		fetchedRole, err := roleService.GetRoleByID(ctx, createdRole.ID)

		assert.NoError(t, err)
		assert.Equal(t, createdRole.ID, fetchedRole.ID)
	})

	suite.Run("Error_GetRoleByID", func() {

		fetchedRole, err := roleService.GetRoleByID(ctx, -1) // Invalid ID

		assert.Error(t, err)
		assert.Nil(t, fetchedRole)
	})

	suite.Run("Normal_UpdateRole", func() {

		newRole := model.NewRole{Name: "Test Role"}
		createdRole, _ := roleService.CreateRole(ctx, newRole)

		updateRole := model.NewRole{Name: "Updated Role"}

		updatedRole, err := roleService.UpdateRole(ctx, createdRole.ID, updateRole)

		assert.NoError(t, err)
		assert.Equal(t, "Updated Role", updatedRole.Name)
	})

	suite.Run("Error_UpdateRole", func() {

		updateRole := model.NewRole{Name: "Updated Role"}

		updatedRole, err := roleService.UpdateRole(ctx, -1, updateRole) // Invalid ID

		assert.Error(t, err)
		assert.Nil(t, updatedRole)
	})

	suite.Run("Normal_DeleteRole", func() {

		newRole := model.NewRole{Name: "Test Role"}
		createdRole, _ := roleService.CreateRole(ctx, newRole)

		deleted, err := roleService.DeleteRole(ctx, createdRole.ID)

		assert.NoError(t, err)
		assert.True(t, *deleted)
	})

	suite.Run("Error_DeleteRole", func() {

		deleted, err := roleService.DeleteRole(ctx, -1) // Invalid ID

		assert.Error(t, err)
		assert.False(t, *deleted)
	})

	suite.Run("Normal_AssignRoleToUser", func() {

		// Create a user and role
		newUser := model.NewUser{Name: "Test User", Created: time.Now(), Updated: time.Now()}
		createdUser, _ := userService.CreateUser(ctx, newUser)

		newRole := model.NewRole{Name: "Test Role"}
		createdRole, _ := roleService.CreateRole(ctx, newRole)

		// Assign role to user
		updatedUser, err := roleService.AssignRoleToUser(ctx, createdUser.ID, createdRole.ID)

		assert.NoError(t, err)
		assert.Equal(t, createdUser.ID, updatedUser.ID)
	})

	suite.Run("Error_AssignRoleToUser", func() {

		updatedUser, err := roleService.AssignRoleToUser(ctx, -1, -1) // Invalid IDs

		assert.Error(t, err)
		assert.Nil(t, updatedUser)
	})

	suite.Run("Normal_RemoveRoleFromUser", func() {

		// Create a user and role
		newUser := model.NewUser{Name: "Test User", Created: time.Now(), Updated: time.Now()}
		createdUser, _ := userService.CreateUser(ctx, newUser)

		newRole := model.NewRole{Name: "Test Role"}
		createdRole, _ := roleService.CreateRole(ctx, newRole)

		// Assign role to user
		roleService.AssignRoleToUser(ctx, createdUser.ID, createdRole.ID)

		// Remove role from user
		updatedUser, err := roleService.RemoveRoleFromUser(ctx, createdUser.ID, createdRole.ID)

		assert.NoError(t, err)
		assert.Equal(t, createdUser.ID, updatedUser.ID)
	})

	suite.Run("Error_RemoveRoleFromUser", func() {

		updatedUser, err := roleService.RemoveRoleFromUser(ctx, -1, -1) // Invalid IDs

		assert.Error(t, err)
		assert.Nil(t, updatedUser)
	})

	suite.Run("Normal_ListRoles", func() {

		newRole1 := model.NewRole{Name: "Test Role 1"}
		newRole2 := model.NewRole{Name: "Test Role 2"}
		roleService.CreateRole(ctx, newRole1)
		roleService.CreateRole(ctx, newRole2)

		roles, err := roleService.Roles(ctx)

		assert.NoError(t, err)
		assert.Len(t, roles, 2)
	})
}

func TestRoleTestSuite(t *testing.T) {
	suite.Run(t, new(RoleTestSuite))
}
