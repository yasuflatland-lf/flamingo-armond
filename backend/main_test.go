package main

import (
	"backend/graph"
	"context"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4/middleware"
	"net/http"
	"net/http/httptest"
	"testing"

	"backend/pkg/config"
	"backend/testutils"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestMainSmoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := "test"
	password := "test"
	dbName := "test"

	// Setup the test database
	pg, cleanup, err := testutils.SetupTestDB(ctx, user, password, dbName)
	if err != nil {
		t.Fatalf("Failed to set up test database: %+v", err)
	}
	defer cleanup(migrationFilePath)

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

	// Initialize Echo
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Create a new resolver with the database connection
	resolver := &graph.Resolver{
		DB: pg.GetDB(),
	}

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	// Setup WebSocket for subscriptions
	srv.AddTransport(&transport.Websocket{
		Upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	})

	e.GET("/", func(c echo.Context) error {
		playground.Handler("GraphQL playground", "/query").ServeHTTP(c.Response(), c.Request())
		return nil
	})

	e.POST("/query", func(c echo.Context) error {
		srv.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	// Create a test server
	ts := httptest.NewServer(e)
	defer ts.Close()

	// Make a simple GET request to the playground
	res, err := http.Get(ts.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)
}
