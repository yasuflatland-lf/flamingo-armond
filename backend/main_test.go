package main

import (
	"backend/pkg/config"
	"backend/testutils"
	"backend/web/server"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setupTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	ctx := context.Background()
	user := "test"
	password := "test"
	dbName := "test"

	// Setup the test database
	pg, cleanup, err := testutils.SetupTestDB(ctx, user, password, dbName)
	if err != nil {
		t.Fatalf("Failed to set up test database: %+v", err)
	}

	// Run migrations
	if err := pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		t.Fatalf("failed to run migrations: %+v", err)
	}

	// Override the config values for testing
	config.Cfg.PGHost = pg.GetConfig().Host
	config.Cfg.PGUser = pg.GetConfig().User
	config.Cfg.PGPassword = pg.GetConfig().Password
	config.Cfg.PGDBName = pg.GetConfig().DBName
	config.Cfg.PGPort = pg.GetConfig().Port
	config.Cfg.PGSSLMode = "disable"
	config.Cfg.Port = 8080

	// Initialize the Echo router using the NewRouter function
	e := server.NewRouter(pg.GetDB())

	ts := httptest.NewServer(e)

	return ts, func() {
		ts.Close()
		cleanup(migrationFilePath)
	}
}

func TestMainSmoke(t *testing.T) {
	t.Parallel()

	ts, cleanup := setupTestServer(t)
	defer cleanup()

	// Make a simple GET request to the playground
	res, err := http.Get(ts.URL + "/health")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}
