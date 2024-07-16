package repository

import (
	"backend/pkg/config"
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var migrationFilePath = "../../db/migrations"

// setupTestDB sets up a Postgres test container and returns the connection and a cleanup function.
func setupTestDB(ctx context.Context, dbName string) (Repository, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     config.Cfg.PGUser,
			"POSTGRES_PASSWORD": config.Cfg.PGPassword,
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
		err := pgContainer.Terminate(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		err := pgContainer.Terminate(ctx)
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("failed to get container port: %w", err)
	}

	config := DBConfig{
		Host:     host,
		User:     config.Cfg.PGUser,
		Password: config.Cfg.PGPassword,
		DBName:   dbName,
		Port:     port.Port(),
		SSLMode:  "disable",
	}

	pg := NewPostgres(config)
	if err = pg.Open(); err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	cleanup := func() {
		// Cleanup database
		if err := pg.RunGooseMigrationsDown(migrationFilePath); err != nil {
			log.Fatalf("failed to run migrations: %+v", err)
		}

		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate postgres container: %+v", err)
		}
	}

	return pg, cleanup, nil
}

// TestPostgres_Open tests the opening of a Postgres connection and running of migrations.
func TestPostgres_Open(t *testing.T) {
	ctx := context.Background()

	pg, cleanup, err := setupTestDB(ctx, config.Cfg.PGDBName)
	if err != nil {
		t.Fatalf("setupTestDB failed: %s", err)
	}
	defer cleanup()

	sqlDB, err := pg.GetDB().DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %s", err)
	}

	if err = sqlDB.Ping(); err != nil {
		t.Fatalf("failed to ping database: %s", err)
	}

	if err = pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		t.Fatalf("goose migration failed: %+v", err)
	}

	t.Logf("Database connection established and migrations ran successfully!")
}

// TestPostgres_RunGooseMigrations tests running Goose migrations on the Postgres database.
func TestPostgres_RunGooseMigrations(t *testing.T) {
	ctx := context.Background()

	pg, cleanup, err := setupTestDB(ctx, config.Cfg.PGDBName)
	if err != nil {
		t.Fatalf("setupTestDB failed: %s", err)
	}
	defer cleanup()

	if err = pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		t.Fatalf("goose migration failed: %s", err)
	}

	t.Logf("Goose migrations ran successfully!")
}
