package server

import (
	"backend/graph"
	"backend/graph/services"
	"backend/pkg/config"
	"backend/pkg/middlewares"
	"backend/pkg/repository"
	"backend/pkg/validator"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"log"
	"net/http"
	"strconv"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

func NewRouter(db *gorm.DB) *echo.Echo {

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Store the database connection in Echo's context
	e.Use(middlewares.DatabaseCtxMiddleware(db))

	// Set Transaction Middleware
	e.Use(middlewares.TransactionMiddleware())

	// Create services
	service := services.New(db)

	// Validator
	validateWrapper := validator.NewValidateWrapper()

	// Create a new resolver
	resolver := &graph.Resolver{
		DB:      db,
		Srv:     service,
		VW:      validateWrapper,
		Loaders: graph.NewLoaders(service),
	}

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	// GraphQL Complexity configuration
	srv.Use(extension.FixedComplexityLimit(config.Cfg.GQLComplexity))

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

	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	e.POST("/query", func(c echo.Context) error {
		srv.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	return e
}

func StartServer(dbConfig repository.DBConfig) {
	// Initialize the database
	db := repository.InitializeDatabase(dbConfig)

	router := NewRouter(db)

	router.Logger.Fatal(router.Start(":" + strconv.Itoa(config.Cfg.Port)))

	log.Printf("connect to http://localhost:%d/ for GraphQL playground", config.Cfg.Port)
}
