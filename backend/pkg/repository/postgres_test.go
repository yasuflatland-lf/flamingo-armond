package repository

import (
	"backend/pkg/config"
	"context"
	"fmt"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq" // Importing the PostgreSQL driver
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"log"
	"net"
	"testing"
	"time"
)

var migrationFilePath = "../../db/migrations"

// setupTestDB sets up a Postgres test container and returns the connection and a cleanup function.
func setupTestDB(ctx context.Context, dbName string) (Repository, func(), error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:16",
		postgres.WithDatabase(dbName),
		postgres.WithUsername(config.Cfg.PGUser),
		postgres.WithPassword(config.Cfg.PGPassword),
		testcontainers.WithEnv(
			map[string]string{
				"POSTGRES_USER":     config.Cfg.PGUser,
				"POSTGRES_PASSWORD": config.Cfg.PGPassword,
				"POSTGRES_DB":       dbName,
			},
		),
		testcontainers.WithWaitStrategy(
			wait.ForSQL("5432", "postgres", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://%s:%s@%s/postgres?sslmode=disable",
					config.Cfg.PGUser, config.Cfg.PGPassword, net.JoinHostPort(host, port.Port()))
			}).WithQuery("select 1 from pg_stat_activity limit 1").WithPollInterval(1*time.Second).WithStartupTimeout(10*time.Second)),
	)
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
