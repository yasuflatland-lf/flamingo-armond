package graph

import (
	"backend/pkg/config"
	"backend/testutils"
	"bytes"
	"context"
	"encoding/json"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

var e *echo.Echo
var migrationFilePath = "../db/migrations"

func TestMain(m *testing.M) {

	// Setup context
	ctx := context.Background()

	// Set up the test database
	pg, cleanup, err := testutils.SetupTestDB(ctx, config.Cfg.PGUser, config.Cfg.PGPassword, config.Cfg.PGDBName)
	if err != nil {
		log.Fatalf("Failed to setup test database: %v", err)
	}
	defer cleanup()

	// Run migrations
	if err := pg.RunGooseMigrations(migrationFilePath); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Setup Echo server
	e = setupEchoServer(pg.DB)

	// Run the tests
	m.Run()
}

func setupEchoServer(db *gorm.DB) *echo.Echo {
	resolver := &Resolver{DB: db}
	srv := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolver}))

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", func(c echo.Context) error {
		playground.Handler("GraphQL playground", "/query").ServeHTTP(c.Response(), c.Request())
		return nil
	})

	e.POST("/query", func(c echo.Context) error {
		srv.ServeHTTP(c.Response(), c.Request())
		return nil
	})

	return e
}

func TestGraphQLResolvers(t *testing.T) {

	t.Run("CreateCard", func(t *testing.T) {
		input := map[string]interface{}{
			"input": map[string]interface{}{
				"front":         "front",
				"back":          "back",
				"review_date":   "2024-07-10",
				"interval_days": 1,
				"cardgroup_id":  "1",
			},
		}
		query := `mutation ($input: NewCard!) {
			createCard(input: $input) {
				id
				front
				back
				review_date
				interval_days
				cardgroup_id
			}
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"createCard":{"front":"front","back":"back","review_date":"2024-07-10","interval_days":1,"cardgroup_id":"1"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("UpdateCard", func(t *testing.T) {
		input := map[string]interface{}{
			"id": "1",
			"input": map[string]interface{}{
				"front":         "updated front",
				"back":          "updated back",
				"review_date":   "2024-08-10",
				"interval_days": 2,
				"cardgroup_id":  "1",
			},
		}
		query := `mutation ($id: ID!, $input: NewCard!) {
			updateCard(id: $id, input: $input) {
				id
				front
				back
				review_date
				interval_days
				cardgroup_id
			}
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"updateCard":{"front":"updated front","back":"updated back","review_date":"2024-08-10","interval_days":2,"cardgroup_id":"1"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("DeleteCard", func(t *testing.T) {
		query := `mutation ($id: ID!) { deleteCard(id: $id) }`
		input := map[string]interface{}{
			"id": "1",
		}
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"deleteCard":true}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("CreateUser", func(t *testing.T) {
		input := map[string]interface{}{
			"input": map[string]interface{}{
				"name": "John Doe",
			},
		}
		query := `mutation ($input: NewUser!) {
			createUser(input: $input) {
				id
				name
			}
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"createUser":{"name":"John Doe"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("UpdateUser", func(t *testing.T) {
		input := map[string]interface{}{
			"id":   "1",
			"name": "Jane Doe",
		}
		query := `mutation ($id: ID!, $name: String!) {
			updateUser(id: $id, name: $name) {
				id
				name
			}
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"updateUser":{"name":"Jane Doe"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("DeleteUser", func(t *testing.T) {
		query := `mutation ($id: ID!) { deleteUser(id: $id) }`
		input := map[string]interface{}{
			"id": "1",
		}
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"deleteUser":true}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("CreateCardGroup", func(t *testing.T) {
		input := map[string]interface{}{
			"input": map[string]interface{}{
				"name": "group1",
			},
		}
		query := `mutation ($input: NewCardGroup!) {
			createCardGroup(input: $input) {
				id
				name
			}
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"createCardGroup":{"name":"group1"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("UpdateCardGroup", func(t *testing.T) {
		input := map[string]interface{}{
			"id":   "1",
			"name": "group2",
		}
		query := `mutation ($id: ID!, $name: String!) {
			updateCardGroup(id: $id, name: $name) {
				id
				name
			}
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"updateCardGroup":{"name":"group2"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("DeleteCardGroup", func(t *testing.T) {
		query := `mutation ($id: ID!) { deleteCardGroup(id: $id) }`
		input := map[string]interface{}{
			"id": "1",
		}
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"deleteCardGroup":true}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("CreateRole", func(t *testing.T) {
		input := map[string]interface{}{
			"input": map[string]interface{}{
				"name": "admin",
			},
		}
		query := `mutation ($input: NewRole!) {
			createRole(input: $input) {
				id
				name
			}
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"createRole":{"name":"admin"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("UpdateRole", func(t *testing.T) {
		input := map[string]interface{}{
			"id":   "1",
			"name": "user",
		}
		query := `mutation ($id: ID!, $name: String!) {
			updateRole(id: $id, name: $name) {
				id
				name
			}
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"updateRole":{"name":"user"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("DeleteRole", func(t *testing.T) {
		query := `mutation ($id: ID!) { deleteRole(id: $id) }`
		input := map[string]interface{}{
			"id": "1",
		}
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"deleteRole":true}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("AddUserToCardGroup", func(t *testing.T) {
		input := map[string]interface{}{
			"userId":      "1",
			"cardGroupId": "1",
		}
		query := `mutation ($userId: ID!, $cardGroupId: ID!) {
			addUserToCardGroup(userId: $userId, cardGroupId: $cardGroupId) 
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"addUserToCardGroup":true}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("RemoveUserFromCardGroup", func(t *testing.T) {
		input := map[string]interface{}{
			"userId":      "1",
			"cardGroupId": "1",
		}
		query := `mutation ($userId: ID!, $cardGroupId: ID!) {
			removeUserFromCardGroup(userId: $userId, cardGroupId: $cardGroupId) 
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"removeUserFromCardGroup":true}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("AssignRoleToUser", func(t *testing.T) {
		input := map[string]interface{}{
			"userId": "1",
			"roleId": "1",
		}
		query := `mutation ($userId: ID!, $roleId: ID!) {
			assignRoleToUser(userId: $userId, roleId: $roleId) 
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"assignRoleToUser":true}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("RemoveRoleFromUser", func(t *testing.T) {
		input := map[string]interface{}{
			"userId": "1",
			"roleId": "1",
		}
		query := `mutation ($userId: ID!, $roleId: ID!) {
			removeRoleFromUser(userId: $userId, roleId: $roleId) 
		}`
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"removeRoleFromUser":true}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("Cards", func(t *testing.T) {
		query := `query {
			cards {
				id
				front
				back
				review_date
				interval_days
				cardgroup_id
			}
		}`
		expected := `{"data":{"cards":[{"id":"1","front":"front","back":"back","review_date":"2024-07-10","interval_days":1,"cardgroup_id":"1"}]}}`
		testGraphQLQuery(t, e, query, nil, expected)
	})

	t.Run("Card", func(t *testing.T) {
		query := `query ($id: ID!) {
			card(id: $id) {
				id
				front
				back
				review_date
				interval_days
				cardgroup_id
			}
		}`
		input := map[string]interface{}{
			"id": "1",
		}
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"card":{"id":"1","front":"front","back":"back","review_date":"2024-07-10","interval_days":1,"cardgroup_id":"1"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("Users", func(t *testing.T) {
		query := `query {
			users {
				id
				name
			}
		}`
		expected := `{"data":{"users":[{"id":"1","name":"John Doe"}]}}`
		testGraphQLQuery(t, e, query, nil, expected)
	})

	t.Run("User", func(t *testing.T) {
		query := `query ($id: ID!) {
			user(id: $id) {
				id
				name
			}
		}`
		input := map[string]interface{}{
			"id": "1",
		}
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"user":{"id":"1","name":"John Doe"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("CardGroups", func(t *testing.T) {
		query := `query {
			cardGroups {
				id
				name
			}
		}`
		expected := `{"data":{"cardGroups":[{"id":"1","name":"group1"}]}}`
		testGraphQLQuery(t, e, query, nil, expected)
	})

	t.Run("CardGroup", func(t *testing.T) {
		query := `query ($id: ID!) {
			cardGroup(id: $id) {
				id
				name
			}
		}`
		input := map[string]interface{}{
			"id": "1",
		}
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"cardGroup":{"id":"1","name":"group1"}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})

	t.Run("Roles", func(t *testing.T) {
		query := `query {
			roles {
				id
				name
				users {
					id
					name
				}
			}
		}`
		expected := `{"data":{"roles":[{"id":"1","name":"admin","users":[{"id":"1","name":"John Doe"}]}]}}`
		testGraphQLQuery(t, e, query, nil, expected)
	})

	t.Run("Role", func(t *testing.T) {
		query := `query ($id: ID!) {
			role(id: $id) {
				id
				name
				users {
					id
					name
				}
			}
		}`
		input := map[string]interface{}{
			"id": "1",
		}
		jsonInput, _ := json.Marshal(input)
		expected := `{"data":{"role":{"id":"1","name":"admin","users":[{"id":"1","name":"John Doe"}]}}}`
		testGraphQLQuery(t, e, query, jsonInput, expected)
	})
}

func testGraphQLQuery(t *testing.T, e *echo.Echo, query string, variables []byte, expected string) {
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBuffer(variables))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.JSONEq(t, expected, rec.Body.String())
}
