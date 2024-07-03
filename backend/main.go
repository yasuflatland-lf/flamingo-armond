package main

import (
	"backend/graph"
	"log"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const defaultPort = "8080"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}}))

	e.GET("/", func(c echo.Context) error {
		playground.Handler("GraphQL playground", "/query").ServeHTTP(c.Response(), c.Request())
		return nil
	})

	e.POST("/query", func(c echo.Context) error {
		srv.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(e.Start(":" + port))
}
