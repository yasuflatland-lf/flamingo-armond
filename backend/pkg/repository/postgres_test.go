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

func TestPostgres_Open(t *testing.T) {
	ctx := context.Background()

	// Create the PostgreSQL container
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
		t.Fatalf("failed to start postgres container: %s", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate postgres container: %s", err)
		}
	}()

	host, err := pgContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %s", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get container port: %s", err)
	}

	// Set up the Postgres configuration
	config := DBConfig{
		Host:     host,
		User:     "testuser",
		Password: "testpassword",
		DBName:   "testdb",
		Port:     port.Port(),
		SSLMode:  "disable",
	}

	// Initialize the Postgres struct
	pg := NewPostgres(config)

	// Open the database connection
	err = pg.Open()
	if err != nil {
		t.Fatalf("failed to open database: %s", err)
	}

	// Verify the connection
	sqlDB, err := pg.DB.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %s", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		t.Fatalf("failed to ping database: %s", err)
	}

	// Run Goose migrations (if needed)
	// err = pg.runGooseMigrations()
	// if err != nil {
	// 	t.Fatalf("goose migration failed: %s", err)
	// }

	fmt.Println("Database connection established successfully!")
}

func TestPostgres_runGooseMigrations(t *testing.T) {
	ctx := context.Background()

	// Create the PostgreSQL container
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
		t.Fatalf("failed to start postgres container: %s", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate postgres container: %s", err)
		}
	}()

	host, err := pgContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %s", err)
	}

	port, err := pgContainer.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("failed to get container port: %s", err)
	}

	// Set up the Postgres configuration
	config := DBConfig{
		Host:     host,
		User:     "testuser",
		Password: "testpassword",
		DBName:   "testdb",
		Port:     port.Port(),
		SSLMode:  "disable",
	}

	// Initialize the Postgres struct
	pg := NewPostgres(config)

	// Open the database connection
	err = pg.Open()
	if err != nil {
		t.Fatalf("failed to open database: %s", err)
	}

	// Run Goose migrations
	err = pg.runGooseMigrations()
	if err != nil {
		t.Fatalf("goose migration failed: %s", err)
	}

	fmt.Println("Goose migrations ran successfully!")
}
