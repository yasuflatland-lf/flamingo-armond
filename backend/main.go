package main

import (
	"backend/graph"
	"backend/pkg/config"
	"backend/pkg/repository"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

var migrationFilePath = "./db/migrations"

func main() {

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Initialize the database
	db := initializeDatabase()

	// Create a new resolver with the database connection
	resolver := &graph.Resolver{
		DB: db,
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

	err := e.Start(":" + strconv.Itoa(config.Cfg.Port))
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("connect to http://localhost:%d/ for GraphQL playground", config.Cfg.Port)
}

// initializeDatabase encapsulates the database configuration and initialization logic
func initializeDatabase() *gorm.DB {
	// Database configuration
	dbConfig := repository.DBConfig{
		Host:     config.Cfg.PGHost,
		User:     config.Cfg.PGUser,
		Password: config.Cfg.PGPassword,
		DBName:   config.Cfg.PGDBName,
		Port:     config.Cfg.PGPort,
		SSLMode:  config.Cfg.PGSSLMode,
	}

	// Initialize the Postgres instance
	pg := repository.NewPostgres(dbConfig)

	// Open the database connection
	if err := pg.Open(); err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	// Get Full path to the migration DB file.
	fullPath, err := getFullPath(migrationFilePath)
	if err != nil {
		log.Fatalf("Failed to get full path to the migration db file : %+v", err)
	}

	// Run migrations
	log.Printf("Data Migration start ===============")
	if err := pg.RunGooseMigrations(fullPath); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Printf("Data Migration Done ===============")

	return pg.DB
}

// getFullPath takes a relative path and returns the full absolute path
func getFullPath(migrationFilePath string) (string, error) {
	cleanedPath := filepath.Clean(migrationFilePath)
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(currentDir, cleanedPath)
	return fullPath, nil
}
