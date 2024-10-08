package testutils

import (
	repo "backend/graph/db"
	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/config"
	"backend/pkg/repository"
	"context"
	"fmt"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq" // Importing the PostgreSQL driver
	"github.com/m-mizutani/goerr"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
	"log"
	"net"
	"testing"
	"time"
)

// SetupTestDB sets up a Postgres test container and returns the connection and a cleanup function.
func SetupTestDB(ctx context.Context, user, password, dbName string) (repository.Repository, func(migrationFilePath string), error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(config.Cfg.PGUser),
		postgres.WithPassword(config.Cfg.PGPassword),
		testcontainers.WithEnv(
			map[string]string{
				"POSTGRES_USER":     user,
				"POSTGRES_PASSWORD": password,
				"POSTGRES_DB":       dbName,
			},
		),
		testcontainers.WithWaitStrategy(
			wait.ForSQL("5432", "postgres", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://%s:%s@%s/postgres?sslmode=disable",
					user, password, net.JoinHostPort(host, port.Port()))
			}).WithQuery("select 1 from pg_stat_activity limit 1").WithPollInterval(1*time.Second).WithStartupTimeout(10*time.Second)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	host, err := pgContainer.Host(ctx)
	if err != nil {
		pgContainer.Terminate(ctx)
		return nil, nil, goerr.Wrap(err, "failed to get container host")
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		pgContainer.Terminate(ctx)
		return nil, nil, goerr.Wrap(err, "failed to get container port")
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
		return nil, nil, goerr.Wrap(err, "failed to open database: %w", err)
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
	roleService services.RoleService) (*model.CardGroup, *model.User, error) {

	randstr := CryptoRandString(8)

	// Create a role
	newRole := model.NewRole{
		Name: "Test Role" + randstr,
	}
	createdRole, err := roleService.CreateRole(ctx, newRole)
	if err != nil {
		return nil, nil, goerr.Wrap(err)
	}

	// Create a user
	newUser := model.NewUser{
		Name:     "Test User" + randstr,
		Email:    GetRandomEmail(8),
		GoogleID: GenerateUUIDv7(),
		Created:  time.Now().UTC(),
		Updated:  time.Now().UTC(),
		RoleIds:  []int64{createdRole.ID}, // Assign the new role to the user
	}
	createdUser, err := userService.CreateUser(ctx, newUser)
	if err != nil {
		return nil, nil, goerr.Wrap(err)
	}

	// Create a card group
	input := model.NewCardGroup{
		Name:    "Test Group" + randstr,
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
		UserIds: []int64{createdUser.ID},
	}

	createdCardGroup, err := cardGroupService.CreateCardGroup(ctx, input)
	if err != nil {
		return nil, nil, goerr.Wrap(err)
	}

	_, err = cardGroupService.AddUserToCardGroup(ctx, createdUser.ID, createdCardGroup.ID)
	if err != nil {
		return nil, nil, goerr.Wrap(err)
	}

	return createdCardGroup, createdUser, nil
}

func CreateUserCardAndCardGroup(
	ctx context.Context,
	userService services.UserService,
	cardGroupService services.CardGroupService,
	roleService services.RoleService,
	cardService services.CardService,
) (*model.Card, *model.CardGroup, *model.User, error) {

	// Step 1: Create User and Card Group using the existing function
	cardGroup, user, err := CreateUserAndCardGroup(ctx, userService, cardGroupService, roleService)
	if err != nil {
		return nil, nil, nil, goerr.Wrap(err, "failed to create user and card group")
	}

	// Step 2: Create a Card
	randstr := CryptoRandString(8)
	if err != nil {
		return nil, nil, nil, goerr.Wrap(err, "failed to generate random string")
	}

	intervalDays := 1

	newCard := model.NewCard{
		Front:        "Front of Card " + randstr,
		Back:         "Back of Card " + randstr,
		ReviewDate:   time.Now().UTC(),
		IntervalDays: &intervalDays, // Set pointer directly
		CardgroupID:  cardGroup.ID,
		Created:      time.Now().UTC(),
		Updated:      time.Now().UTC(),
	}

	createdCard, err := cardService.CreateCard(ctx, newCard)
	if err != nil {
		return nil, nil, nil, goerr.Wrap(err, "failed to create card")
	}

	return createdCard, cardGroup, user, nil
}
