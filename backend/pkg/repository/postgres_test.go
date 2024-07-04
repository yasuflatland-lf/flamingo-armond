package repository

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(ctx context.Context) (*Postgres, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:13",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "testuser",
			"POSTGRES_PASSWORD": "testpassword",
			"POSTGRES_DB":       "testdb",
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
		User:     "testuser",
		Password: "testpassword",
		DBName:   "testdb",
		Port:     port.Port(),
		SSLMode:  "disable",
	}

	pg := NewPostgres(config)
	err = pg.Open()
	if err != nil {
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

func TestPostgres_Open(t *testing.T) {
	ctx := context.Background()

	pg, cleanup, err := setupTestDB(ctx)
	if err != nil {
		t.Fatalf("setupTestDB failed: %s", err)
	}
	defer cleanup()

	sqlDB, err := pg.DB.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %s", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		t.Fatalf("failed to ping database: %s", err)
	}

	err = pg.runGooseMigrations()
	if err != nil {
		t.Fatalf("goose migration failed: %s", err)
	}

	fmt.Println("Database connection established successfully!")
}

func TestPostgres_runGooseMigrations(t *testing.T) {
	ctx := context.Background()

	pg, cleanup, err := setupTestDB(ctx)
	if err != nil {
		t.Fatalf("setupTestDB failed: %s", err)
	}
	defer cleanup()

	err = pg.runGooseMigrations()
	if err != nil {
		t.Fatalf("goose migration failed: %s", err)
	}

	fmt.Println("Goose migrations ran successfully!")
}
