package main

import (
	"backend/pkg/config"
	"backend/pkg/utils"
	"backend/testutils"
	"backend/web/server"
	"context"
	"fmt"
	"github.com/m-mizutani/goerr"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func setupTestDB(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	ctx := context.Background()

	// Setup the test database
	pg, cleanup, err := testutils.SetupTestDB(ctx, config.Cfg.PGUser, config.Cfg.PGPassword, config.Cfg.PGDBName)
	if err != nil {
		t.Fatalf("Failed to set up test database: %+v", err)
	}

	// Get Full path to the migration DB file.
	fullPath, err := utils.GetFullPath(migrationFilePath)
	if err != nil {
		log.Fatalf("Failed to get full path to the migration db file : %+v", err)
	}

	// Run migrations
	if err := pg.RunGooseMigrationsUp(fullPath); err != nil {
		t.Fatalf("failed to run migrations: %+v", err)
	}

	// Initialize the Echo router using the NewRouter function
	e := server.NewRouter(pg.GetDB())

	ts := httptest.NewServer(e)

	return ts, func() {
		ts.Close()
		cleanup(migrationFilePath)
	}
}

func setupProdDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		config.Cfg.PGHost,
		config.Cfg.PGUser,
		config.Cfg.PGPassword,
		config.Cfg.PGDBName,
		config.Cfg.PGPort,
		config.Cfg.PGSSLMode)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, goerr.Wrap(err)
	}
	return db, nil
}

func setupProdServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	// Set up the production database
	db, err := setupProdDB()
	if err != nil {
		t.Fatalf("Failed to connect to production database: %+v", err)
	}

	// Initialize the Echo router using the NewRouter function
	e := server.NewRouter(db)

	ts := httptest.NewServer(e)

	return ts, func() {
		ts.Close()
		// Add cleanup code if necessary
	}
}

func TestMainSmoke(t *testing.T) {
	t.Parallel()

	t.Run("Test with Mock Database", func(t *testing.T) {
		// Comment out this when you want to do a smoke test against production database.
		//t.SkipNow()
		ts, cleanup := setupTestDB(t)
		defer cleanup()

		// Make a simple GET request to the playground
		res, err := http.Get(ts.URL + "/health")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})

	t.Run("Test with Production Database", func(t *testing.T) {
		// Comment out this when you want to do a smoke test against production database.
		t.SkipNow()
		ts, cleanup := setupProdServer(t)
		defer cleanup()

		// Make a simple GET request to the playground
		res, err := http.Get(ts.URL + "/health")
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, res.StatusCode)
	})
}
