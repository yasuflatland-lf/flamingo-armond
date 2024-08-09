package testutils

import (
	"backend/graph/model"
	"backend/graph/services"
	"context"
	"fmt"
	"gorm.io/gorm"
	"log"
	"testing"
	"time"

	repo "backend/graph/db"
	"backend/pkg/repository"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SetupTestDB sets up a Postgres test container and returns the connection and a cleanup function.
func SetupTestDB(ctx context.Context, user, password, dbName string) (repository.Repository, func(migrationFilePath string), error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     user,
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       dbName,
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(5 * time.Minute),
	}

	pgContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to get container port: %w", err)
	}

	config := repository.DBConfig{
		Host:     host,
		User:     user,
		Password: password,
		DBName:   dbName,
		Port:     port.Port(),
		SSLMode:  "disable",
	}

	pg := repository.NewPostgres(config)
	if err = pg.Open(); err != nil {
		pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	cleanup := func(migrationFilePath string) {
		// Clean up database
		if err := pg.RunGooseMigrationsDown(migrationFilePath); err != nil {
			log.Fatalf("failed to run migrations: %v", err)
		}

		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate postgres container: %s", err)
		}
	}

	return pg, cleanup, nil
}

func RunServersTest(t *testing.T, db *gorm.DB, fn func(*testing.T)) {
	// Begin a new transaction
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("Failed to begin transaction: %v", tx.Error)
	}

	// Disable foreign key constraints
	if err := tx.Exec("SET CONSTRAINTS ALL DEFERRED").Error; err != nil {
		t.Fatalf("Failed to disable foreign key constraints: %v", err)
	}

	// Delete records from tables
	tx.Where("1 = 1").Delete(&repo.SwipeRecord{})
	tx.Where("1 = 1").Delete(&repo.Card{})
	tx.Where("1 = 1").Delete(&repo.Cardgroup{})
	tx.Where("1 = 1").Delete(&repo.User{})
	tx.Where("1 = 1").Delete(&repo.Role{})

	// Call the provided test function
	if fn != nil {
		fn(t)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}

	// Enable foreign key constraints
	if err := db.Exec("SET CONSTRAINTS ALL IMMEDIATE").Error; err != nil {
		t.Fatalf("Failed to enable foreign key constraints: %v", err)
	}
}

func CreateUserAndCardGroup(
	ctx context.Context,
	userService services.UserService,
	cardGroupService services.CardGroupService,
	roleService services.RoleService) (*model.CardGroup, error) {

	// Create a role
	newRole := model.NewRole{
		Name: "Test Role",
	}
	createdRole, err := roleService.CreateRole(ctx, newRole)
	if err != nil {
		return nil, err
	}

	// Create a user
	newUser := model.NewUser{
		Name:    "Test User",
		Created: time.Now(),
		Updated: time.Now(),
		RoleIds: []int64{createdRole.ID}, // Assign the new role to the user
	}
	createdUser, err := userService.CreateUser(ctx, newUser)
	if err != nil {
		return nil, err
	}

	// Create a card group
	input := model.NewCardGroup{
		Name:    "Test Group",
		Created: time.Now(),
		Updated: time.Now(),
		UserIds: []int64{createdUser.ID},
	}

	return cardGroupService.CreateCardGroup(ctx, input)
}
