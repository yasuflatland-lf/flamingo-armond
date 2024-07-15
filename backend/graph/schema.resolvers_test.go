package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"backend/graph/model"
	"backend/pkg/config"
	repository "backend/pkg/model"
	"backend/testutils"
)

var e *echo.Echo
var db *gorm.DB
var migrationFilePath = "../db/migrations"

func TestMain(m *testing.M) {
	// Setup context
	ctx := context.Background()

	// Set up the test database
	pg, cleanup, err := testutils.SetupTestDB(ctx, config.Cfg.PGUser, config.Cfg.PGPassword, config.Cfg.PGDBName)
	if err != nil {
		log.Fatalf("Failed to setup test database: %+v", err)
	}
	defer cleanup(migrationFilePath)

	// Run migrations
	if err := pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		log.Fatalf("failed to run migrations: %+v", err)
	}

	// Setup Echo server
	db = pg.DB
	e = setupEchoServer(db)

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

func testGraphQLQuery(t *testing.T, e *echo.Echo, jsonInput []byte, expected string, ignoreFields ...string) {
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBuffer(jsonInput))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	var actualResponse map[string]interface{}
	var expectedResponse map[string]interface{}

	// Parse the actual response
	if err := json.Unmarshal(rec.Body.Bytes(), &actualResponse); err != nil {
		t.Fatalf("Failed to unmarshal actual response: %v", err)
	}

	// Parse the expected response
	if err := json.Unmarshal([]byte(expected), &expectedResponse); err != nil {
		t.Fatalf("Failed to unmarshal expected response: %v", err)
	}

	// Remove the fields to ignore from both responses
	for _, field := range ignoreFields {
		removeField(actualResponse, field)
		removeField(expectedResponse, field)
	}

	// Compare the modified responses
	assert.Equal(t, expectedResponse, actualResponse)
}

func removeField(data map[string]interface{}, field string) {
	parts := strings.Split(field, ".")
	if len(parts) == 1 {
		delete(data, parts[0])
	} else {
		if next, ok := data[parts[0]]; ok {
			switch next := next.(type) {
			case map[string]interface{}:
				removeField(next, strings.Join(parts[1:], "."))
			case []interface{}:
				for _, item := range next {
					if itemMap, ok := item.(map[string]interface{}); ok {
						removeField(itemMap, strings.Join(parts[1:], "."))
					}
				}
			}
		}
	}
}

func runServersTest(t *testing.T, fn func(*testing.T)) {
	// Begin a new transaction
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("Failed to begin transaction: %v", tx.Error)
	}

	// Use a defer statement to ensure the transaction is rolled back if the function exits unexpectedly
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			t.Fatalf("Recovered in runServersTest: %v", r)
		}
	}()

	// Perform the database operations inside the transaction
	if err := tx.Where("1 = 1").Delete(&repository.User{}).Error; err != nil {
		tx.Rollback()
		t.Fatalf("Failed to delete users: %v", err)
	}

	if err := tx.Where("1 = 1").Delete(&repository.Role{}).Error; err != nil {
		tx.Rollback()
		t.Fatalf("Failed to delete roles: %v", err)
	}

	if err := tx.Where("1 = 1").Delete(&repository.Card{}).Error; err != nil {
		tx.Rollback()
		t.Fatalf("Failed to delete cards: %v", err)
	}

	if err := tx.Where("1 = 1").Delete(&repository.Cardgroup{}).Error; err != nil {
		tx.Rollback()
		t.Fatalf("Failed to delete card groups: %v", err)
	}

	// Call the provided test function
	fn(t)

	// Commit the transaction if all operations succeed
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}
}

func TestMutationResolver(t *testing.T) {
	runServersTest(t, func(t *testing.T) {
		t.Run("CreateCard", func(t *testing.T) {

			// Step 1: Create a Cardgroup
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestMutationResolver_CreateCard Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

			// Step 2: Create a new card with the CardgroupID
			input := model.NewCard{
				Front:        "CreateCard Front of card",
				Back:         "CreateCard Back of card",
				ReviewDate:   now,
				IntervalDays: new(int),
				CardgroupID:  cardgroup.ID,
			}
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($input: NewCard!) {
				createCard(input: $input) {
					id
					front
					back
					review_date
					interval_days
					created
					updated
				}
			}`,
				"variables": map[string]interface{}{
					"input": input,
				},
			})
			expected := fmt.Sprintf(`{
			"data": {
				"createCard": {
					"id": 1,
					"front": "CreateCard Front of card",
					"back": "CreateCard Back of card",
					"review_date": "%s",
					"interval_days": 1,
					"created": "%s",
					"updated": "%s"
				}
			}
		}`, cardgroup.Created.Format(time.RFC3339Nano), cardgroup.Created.Format(time.RFC3339Nano), cardgroup.Updated.Format(time.RFC3339Nano))

			testGraphQLQuery(t, e, jsonInput, expected, "data.createCard.id", "data.createCard.created", "data.createCard.updated", "data.createCard.review_date")
		})

		t.Run("CreateCard_Error", func(t *testing.T) {
			t.Parallel()

			// Attempt to create a new card with an invalid CardgroupID
			input := model.NewCard{
				Front:        "CreateCard Front of card",
				Back:         "CreateCard Back of card",
				ReviewDate:   time.Now(),
				IntervalDays: new(int),
				CardgroupID:  -1, // Invalid ID
			}
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($input: NewCard!) {
				createCard(input: $input) {
					id
					front
					back
					review_date
					interval_days
					created
					updated
				}
			}`,
				"variables": map[string]interface{}{
					"input": input,
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UpdateCard", func(t *testing.T) {
			t.Parallel()

			// Step 1: Create a Cardgroup
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestMutationResolver_UpdateCard Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

			// Create a card to update
			card := repository.Card{
				Front:        "UpdateCard Old Front",
				Back:         "UpdateCard Old Back",
				ReviewDate:   time.Now(),
				IntervalDays: 1,
				CardGroupID:  cardgroup.ID,
				Created:      time.Now(),
				Updated:      time.Now(),
			}
			db.Create(&card)

			input := model.NewCard{
				Front:      "UpdateCard New Front",
				Back:       "UpdateCard New Back",
				ReviewDate: time.Now(),
			}
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewCard!) {
				updateCard(id: $id, input: $input) {
					id
					front
					back
					review_date
					interval_days
					created
					updated
				}
			}`,
				"variables": map[string]interface{}{
					"id":    card.ID,
					"input": input,
				},
			})
			expected := `{
			"data": {
				"updateCard": {
					"id": ` + fmt.Sprintf("%d", card.ID) + `,
					"front": "UpdateCard New Front",
					"back": "UpdateCard New Back"
				}
			}
		}`
			testGraphQLQuery(t, e, jsonInput, expected, "data.updateCard.created", "data.updateCard.updated", "data.updateCard.review_date", "data.updateCard.interval_days")
		})

		t.Run("UpdateCard_Error", func(t *testing.T) {
			t.Parallel()

			// Attempt to update a card with an invalid ID
			input := model.NewCard{
				Front:      "UpdateCard New Front",
				Back:       "UpdateCard New Back",
				ReviewDate: time.Now(),
			}
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewCard!) {
				updateCard(id: $id, input: $input) {
					id
					front
					back
					review_date
					interval_days
					created
					updated
				}
			}`,
				"variables": map[string]interface{}{
					"id":    -1, // Invalid ID
					"input": input,
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("DeleteCard", func(t *testing.T) {
			t.Parallel()

			// Step 1: Create a Cardgroup
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestMutationResolver_DeleteCard Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

			// Step 2: Create a card to delete
			card := repository.Card{
				Front:        "DeleteCard Front to delete",
				Back:         "DeleteCard Back to delete",
				ReviewDate:   time.Now(),
				IntervalDays: 1,
				CardGroupID:  cardgroup.ID,
				Created:      time.Now(),
				Updated:      time.Now(),
			}
			db.Create(&card)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
				deleteCard(id: $id)
			}`,
				"variables": map[string]interface{}{
					"id": card.ID,
				},
			})
			expected := `{
			"data": {
				"deleteCard": true
			}
		}`
			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("DeleteCard_Error", func(t *testing.T) {
			t.Parallel()

			// Attempt to delete a card with an invalid ID
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
				deleteCard(id: $id)
			}`,
				"variables": map[string]interface{}{
					"id": -1, // Invalid ID
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})
	})
}

func TestQueryResolver(t *testing.T) {
	runServersTest(t, func(t *testing.T) {
		t.Run("Cards", func(t *testing.T) {

			// Step 1: Create a Cardgroup
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestQueryResolver_Cards Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

			// Step 2: Create a Card associated with the created Cardgroup
			card := repository.Card{
				Front:        "Cards Front",
				Back:         "Cards Back",
				ReviewDate:   now,
				IntervalDays: 1,
				CardGroupID:  cardgroup.ID,
				Created:      now,
				Updated:      now,
			}
			db.Create(&card)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
			cards {
				id
				front
				back
				review_date
				interval_days
				created
				updated
			}
		}`,
			})

			// Execute GraphQL query
			req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBuffer(jsonInput))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			// Check HTTP status code
			assert.Equal(t, http.StatusOK, rec.Code)

			// Parse response body
			var response map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Check number of cards in the response
			cards, ok := response["data"].(map[string]interface{})["cards"].([]interface{})
			if !ok {
				t.Fatalf("Failed to parse cards from response")
			}
			assert.Len(t, cards, 1, "Expected number of cards to be 1")

			// Ensure the card details match what was created
			cardDetails := cards[0].(map[string]interface{})
			assert.Equal(t, "Cards Front", cardDetails["front"])
			assert.Equal(t, "Cards Back", cardDetails["back"])
			assert.Equal(t, float64(1), cardDetails["interval_days"])
		})

		t.Run("Cards_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid field
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
			cards {
				invalid_field
			}
		}`,
			})
			expected := `{
			"errors": [{
				"message": "Cannot query field \"invalid_field\" on type \"Card\"."
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("Card", func(t *testing.T) {

			// Step 1: Create a Cardgroup
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestQueryResolver_Card Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

			// Step 2: Create a Card associated with the created Cardgroup
			card := repository.Card{
				Front:        "Card Front",
				Back:         "Card Back",
				ReviewDate:   now,
				IntervalDays: 1,
				CardGroupID:  cardgroup.ID,
				Created:      now,
				Updated:      now,
			}
			db.Create(&card)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
				card(id: $id) {
					id
					front
					back
					review_date
					interval_days
					created
					updated
				}
			}`,
				"variables": map[string]interface{}{
					"id": card.ID,
				},
			})
			expected := fmt.Sprintf(`{
			"data": {
				"card": {
					"id": %d,
					"front": "Card Front",
					"back": "Card Back"
				}
			}
		}`, card.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.card.created", "data.card.updated", "data.card.review_date", "data.card.interval_days")
		})

		t.Run("Card_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid ID
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
				card(id: $id) {
					id
					front
					back
					review_date
					interval_days
					created
					updated
				}
			}`,
				"variables": map[string]interface{}{
					"id": -1, // Invalid ID
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		// Test for roles
		t.Run("Roles", func(t *testing.T) {

			// Step 1: Create a Role
			role := repository.Role{
				Name: "Test Role",
			}
			db.Create(&role)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
					roles {
						id
						name
					}
			}`})

			// Execute GraphQL query
			req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBuffer(jsonInput))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			// Check HTTP status code
			assert.Equal(t, http.StatusOK, rec.Code)

			// Parse response body
			var response map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Check number of roles in the response
			roles, ok := response["data"].(map[string]interface{})["roles"].([]interface{})
			if !ok {
				t.Fatalf("Failed to parse roles from response")
			}
			assert.Len(t, roles, 1, "Expected number of roles to be 1")

			// Ensure the role details match what was created
			roleDetails := roles[0].(map[string]interface{})
			assert.Equal(t, "Test Role", roleDetails["name"])
		})

		t.Run("Role_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid field
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
			roles {
				invalid_field
			}
		}`,
			})
			expected := `{
			"errors": [{
				"message": "Cannot query field \"invalid_field\" on type \"Role\"."
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		// Test for role by ID
		t.Run("Role", func(t *testing.T) {

			// Step 1: Create a Role
			role := repository.Role{
				Name: "Test Role",
			}
			db.Create(&role)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
				role(id: $id) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"id": role.ID,
				},
			})
			expected := fmt.Sprintf(`{
			"data": {
				"role": {
					"id": %d,
					"name": "Test Role"
				}
			}
		}`, role.ID)

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("Role_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid ID
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
				role(id: $id) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"id": -1, // Invalid ID
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		// Test for users
		t.Run("Users", func(t *testing.T) {

			// Step 1: Create a User
			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
			users {
				id
				name
			}
		}`,
			})

			// Execute GraphQL query
			req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBuffer(jsonInput))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			// Check HTTP status code
			assert.Equal(t, http.StatusOK, rec.Code)

			// Parse response body
			var response map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			// Check number of users in the response
			users, ok := response["data"].(map[string]interface{})["users"].([]interface{})
			if !ok {
				t.Fatalf("Failed to parse users from response")
			}
			assert.Len(t, users, 1, "Expected number of users to be 1")

			// Ensure the user details match what was created
			userDetails := users[0].(map[string]interface{})
			assert.Equal(t, "Test User", userDetails["name"])
		})

		t.Run("Users_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid field
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
			users {
				invalid_field
			}
		}`,
			})
			expected := `{
			"errors": [{
				"message": "Cannot query field \"invalid_field\" on type \"User\"."
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		// Test for user by ID
		t.Run("User", func(t *testing.T) {

			// Step 1: Create a User
			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
				user(id: $id) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"id": user.ID,
				},
			})
			expected := fmt.Sprintf(`{
			"data": {
				"user": {
					"id": %d,
					"name": "Test User"
				}
			}
		}`, user.ID)

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("User_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid ID
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
				user(id: $id) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"id": -1, // Invalid ID
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		// Test for cardsByCardGroup
		t.Run("CardsByCardGroup", func(t *testing.T) {

			// Step 1: Create a Cardgroup
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

			// Step 2: Create a Card associated with the created Cardgroup
			card := repository.Card{
				Front:        "Card Front",
				Back:         "Card Back",
				ReviewDate:   now,
				IntervalDays: 1,
				CardGroupID:  cardgroup.ID,
				Created:      now,
				Updated:      now,
			}
			db.Create(&card)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($cardGroupId: ID!) {
				cardsByCardGroup(cardGroupId: $cardGroupId) {
					id
					front
					back
				}
			}`,
				"variables": map[string]interface{}{
					"cardGroupId": cardgroup.ID,
				},
			})
			expected := fmt.Sprintf(`{
			"data": {
				"cardsByCardGroup": [{
					"id": %d,
					"front": "Card Front",
					"back": "Card Back"
				}]
			}
		}`, card.ID)

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CardsByCardGroup_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid cardGroupId
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($cardGroupId: ID!) {
				cardsByCardGroup(cardGroupId: $cardGroupId) {
					id
					front
					back
				}
			}`,
				"variables": map[string]interface{}{
					"cardGroupId": -1, // Invalid ID
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		// Test for userRole
		t.Run("UserRole", func(t *testing.T) {

			// Step 1: Create a Role
			role := repository.Role{
				Name: "Test Role",
			}
			db.Create(&role)

			// Step 2: Create a User and assign the role
			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)
			db.Model(&user).Association("Roles").Append(&role)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($userId: ID!) {
				userRole(userId: $userId) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"userId": user.ID,
				},
			})
			expected := fmt.Sprintf(`{
			"data": {
				"userRole": {
					"id": %d,
					"name": "Test Role"
				}
			}
		}`, role.ID)

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UserRole_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid userId
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($userId: ID!) {
				userRole(userId: $userId) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"userId": -1, // Invalid ID
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		// Test for cardGroupsByUser
		t.Run("CardGroupsByUser", func(t *testing.T) {

			// Step 1: Create a User
			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)

			// Step 2: Create a Cardgroup and assign the user
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)
			db.Model(&cardgroup).Association("Users").Append(&user)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($userId: ID!) {
				cardGroupsByUser(userId: $userId) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"userId": user.ID,
				},
			})
			expected := fmt.Sprintf(`{
			"data": {
				"cardGroupsByUser": [{
					"id": %d,
					"name": "Test CardGroup"
				}]
			}
		}`, cardgroup.ID)

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CardGroupsByUser_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid userId
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($userId: ID!) {
				cardGroupsByUser(userId: $userId) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"userId": -1, // Invalid ID
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		// Test for usersByRole
		t.Run("UsersByRole", func(t *testing.T) {

			// Step 1: Create a Role
			role := repository.Role{
				Name: "Test Role",
			}
			db.Create(&role)

			// Step 2: Create a User and assign the role
			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)
			db.Model(&user).Association("Roles").Append(&role)

			// Prepare GraphQL query
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($roleId: ID!) {
				usersByRole(roleId: $roleId) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"roleId": role.ID,
				},
			})
			expected := fmt.Sprintf(`{
			"data": {
				"usersByRole": [{
					"id": %d,
					"name": "Test User"
				}]
			}
		}`, user.ID)

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UsersByRole_Error", func(t *testing.T) {
			t.Parallel()

			// Prepare GraphQL query with invalid roleId
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($roleId: ID!) {
				usersByRole(roleId: $roleId) {
					id
					name
				}
			}`,
				"variables": map[string]interface{}{
					"roleId": -1, // Invalid ID
				},
			})
			expected := `{
			"errors": [{
				"message": "record not found"
			}]
		}`

			testGraphQLQuery(t, e, jsonInput, expected)
		})
	})
}
