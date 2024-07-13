package testutils

import (
	"context"
	"fmt"
	"log"
	"time"

	"backend/pkg/repository"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SetupTestDB sets up a Postgres test container and returns the connection and a cleanup function.
func SetupTestDB(ctx context.Context, user, password, dbName string) (*repository.Postgres, func(), error) {
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

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate postgres container: %s", err)
		}
	}

	return pg, cleanup, nil
}
