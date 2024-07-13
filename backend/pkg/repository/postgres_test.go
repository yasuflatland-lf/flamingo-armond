package repository

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"backend/pkg/config"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var migrationFilePath = "../../db/migrations"

// setupTestDB sets up a Postgres test container and returns the connection and a cleanup function.
func setupTestDB(ctx context.Context, dbName string) (*Postgres, func(), error) {
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
		pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to get container host: %w", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		pgContainer.Terminate(ctx)
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
		pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate postgres container: %s", err)
		}
	}

	return pg, cleanup, nil
}

// TestPostgres_Open tests the opening of a Postgres connection and running of migrations.
func TestPostgres_Open(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pg, cleanup, err := setupTestDB(ctx, config.Cfg.PGDBName)
	if err != nil {
		t.Fatalf("setupTestDB failed: %s", err)
	}
	defer cleanup()

	sqlDB, err := pg.DB.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %s", err)
	}

	if err = sqlDB.Ping(); err != nil {
		t.Fatalf("failed to ping database: %s", err)
	}

	if err = pg.RunGooseMigrations(migrationFilePath); err != nil {
		t.Fatalf("goose migration failed: %s", err)
	}

	t.Log("Database connection established and migrations ran successfully!")
}

// TestPostgres_RunGooseMigrations tests running Goose migrations on the Postgres database.
func TestPostgres_RunGooseMigrations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pg, cleanup, err := setupTestDB(ctx, config.Cfg.PGDBName)
	if err != nil {
		t.Fatalf("setupTestDB failed: %s", err)
	}
	defer cleanup()

	if err = pg.RunGooseMigrations(migrationFilePath); err != nil {
		t.Fatalf("goose migration failed: %s", err)
	}

	t.Log("Goose migrations ran successfully!")
}

// TestDatabaseCreation checks if the 'flamingodb' database is created.
func TestDatabaseCreation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Use a temporary database name for this test
	pg, cleanup, err := setupTestDB(ctx, config.Cfg.PGDBName)
	if err != nil {
		t.Fatalf("setupTestDB failed: %s", err)
	}
	defer cleanup()

	sqlDB, err := pg.DB.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %s", err)
	}

	if err = sqlDB.Ping(); err != nil {
		t.Fatalf("failed to ping database: %s", err)
	}

	// Check if the database 'flamingodb' exists
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT datname FROM pg_catalog.pg_database WHERE datname = '%s')", config.Cfg.PGDBName)
	err = sqlDB.QueryRow(query).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check database existence: %s", err)
	}

	if !exists {
		t.Fatalf("database %s does not exist", config.Cfg.PGDBName)
	}

	t.Logf("Database %s exists", config.Cfg.PGDBName)
}
