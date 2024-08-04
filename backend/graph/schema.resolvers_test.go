package graph_test

import (
	"backend/graph"
	repository "backend/graph/db"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/gommon/log"
	"github.com/pkg/errors"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"backend/graph/model"
	"backend/graph/services"
	"backend/pkg/config"
	"backend/pkg/middlewares"
	"backend/pkg/validator"
	"backend/testutils"
)

var e *echo.Echo
var db *gorm.DB
var migrationFilePath = "../db/migrations"

func TestMain(m *testing.M) {
	ctx := context.Background()

	pg, cleanup, err := testutils.SetupTestDB(ctx, config.Cfg.PGUser, config.Cfg.PGPassword, config.Cfg.PGDBName)
	if err != nil {
		log.Fatalf("Failed to setup test database: %+v", err)
	}
	defer cleanup(migrationFilePath)

	if err := pg.RunGooseMigrationsUp(migrationFilePath); err != nil {
		log.Fatalf("failed to run migrations: %+v", err)
	}

	db = pg.GetDB()
	e = NewRouter(db)

	m.Run()
}

func NewRouter(db *gorm.DB) *echo.Echo {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middlewares.DatabaseCtxMiddleware(db))
	e.Use(middlewares.TransactionMiddleware())

	service := services.New(db)
	validateWrapper := validator.NewValidateWrapper()
	resolver := &graph.Resolver{
		DB:      db,
		Srv:     service,
		VW:      validateWrapper,
		Loaders: graph.NewLoaders(service),
	}

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	// GraphQL Complexity configuration
	srv.Use(extension.FixedComplexityLimit(config.Cfg.GQLComplexity))

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

func testGraphQLQuery(t *testing.T, e *echo.Echo, jsonInput []byte, expected string, ignoreFields ...string) {
	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBuffer(jsonInput))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	var actualResponse map[string]interface{}
	var expectedResponse map[string]interface{}

	if err := json.Unmarshal(rec.Body.Bytes(), &actualResponse); err != nil {
		t.Fatalf("Failed to unmarshal actual response: %v", err)
	}

	if err := json.Unmarshal([]byte(expected), &expectedResponse); err != nil {
		t.Fatalf("Failed to unmarshal expected response: %v", err)
	}

	for _, field := range ignoreFields {
		removeField(actualResponse, field)
		removeField(expectedResponse, field)
	}

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

func TestGraphQLQueries(t *testing.T) {
	t.Helper()
	t.Parallel()

	testutils.RunServersTest(t, db, func(t *testing.T) {
		t.Run("Card Query", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			cardGroup := repository.Cardgroup{
				Name:    "Card Query Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			card := repository.Card{
				Front:        "Test Card Front",
				Back:         "Test Card Back",
				ReviewDate:   now,
				IntervalDays: 1,
				CardGroupID:  cardGroup.ID,
				Created:      now,
				Updated:      now,
			}
			db.Create(&card)

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
                        "front": "Test Card Front",
                        "back": "Test Card Back",
                        "interval_days": 1
                    }
                }
            }`, card.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.card.created", "data.card.updated", "data.card.review_date")
		})

		t.Run("CardGroup Query", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
                    cardGroup(id: $id) {
                        id
                        name
                        created
                        updated
                    }
                }`,
				"variables": map[string]interface{}{
					"id": cardGroup.ID,
				},
			})

			expected := fmt.Sprintf(`{
                "data": {
                    "cardGroup": {
                        "id": %d,
                        "name": "Test CardGroup"
                    }
                }
            }`, cardGroup.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.cardGroup.created", "data.cardGroup.updated")
		})

		t.Run("User Query", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "Users Query Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "Users Query Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Users Query Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign a role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
                    user(id: $id) {
                        id
                        name
                        created
                        updated
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
                        "name": "Users Query Test User"
                    }
                }
            }`, user.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.user.created", "data.user.updated")
		})

		t.Run("Role Query", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			var role repository.Role
			if err := db.Where("name = ?", "Role Query Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "Role Query Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
                    role(id: $id) {
                        id
                        name
                        created
                        updated
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
                        "name": "Role Query Test Role"
                    }
                }
            }`, role.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.role.created", "data.role.updated")
		})

		t.Run("Cards Query", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			card := repository.Card{
				Front:        "Test Card Front",
				Back:         "Test Card Back",
				ReviewDate:   now,
				IntervalDays: 1,
				CardGroupID:  cardGroup.ID,
				Created:      now,
				Updated:      now,
			}
			db.Create(&card)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($cardGroupID: ID!, $first: Int) {
                    cardsByCardGroup(cardGroupID: $cardGroupID, first: $first) {
                        nodes {
                            id
                            front
                            back
                            review_date
                            interval_days
                            created
                            updated
                        }
                    }
                }`,
				"variables": map[string]interface{}{
					"cardGroupID": cardGroup.ID,
					"first":       1,
				},
			})

			expected := fmt.Sprintf(`{
                "data": {
                    "cardsByCardGroup": {
                        "nodes": [{
                            "id": %d,
                            "front": "Test Card Front",
                            "back": "Test Card Back",
                            "interval_days": 1
                        }]
                    }
                }
            }`, card.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.cardsByCardGroup.nodes.created", "data.cardsByCardGroup.nodes.updated", "data.cardsByCardGroup.nodes.review_date")
		})

		t.Run("UserRole Query", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "UserRole Query Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "UserRole Query Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign a role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($userID: ID!) {
                    userRole(userID: $userID) {
                        id
                        name
                    }
                }`,
				"variables": map[string]interface{}{
					"userID": user.ID,
				},
			})

			expected := fmt.Sprintf(`{
                "data": {
                    "userRole": {
                        "id": %d,
                        "name": "UserRole Query Test Role"
                    }
                }
            }`, role.ID)

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CardGroupsByUser Query", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "CardGroupsByUser Query Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "CardGroupsByUser Query Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign a role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			// Create a card group
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)
			db.Model(&user).Association("CardGroups").Append(&cardGroup)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($userID: ID!, $first: Int) {
                    cardGroupsByUser(userID: $userID, first: $first) {
                        nodes {
                            id
                            name
                            created
                            updated
                        }
                    }
                }`,
				"variables": map[string]interface{}{
					"userID": user.ID,
					"first":  nil,
				},
			})

			expected := fmt.Sprintf(`{
                "data": {
                    "cardGroupsByUser": {
                        "nodes": [{
                            "id": %d,
                            "name": "Test CardGroup"
                        }]
                    }
                }
            }`, cardGroup.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.cardGroupsByUser.nodes.created", "data.cardGroupsByUser.nodes.updated")
		})

		t.Run("UsersByRole Query", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "UsersByRole Query Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "UsersByRole Query Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign a role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($roleID: ID!, $first: Int) {
                    usersByRole(roleID: $roleID, first: $first) {
                        nodes {
                            id
                            name
                            created
                            updated
                        }
                    }
                }`,
				"variables": map[string]interface{}{
					"roleID": role.ID,
					"first":  1,
				},
			})

			expected := fmt.Sprintf(`{
                "data": {
                    "usersByRole": {
                        "nodes": [{
                            "id": %d,
                            "name": "Test User"
                        }]
                    }
                }
            }`, user.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.usersByRole.nodes.created", "data.usersByRole.nodes.updated")
		})

		t.Run("CreateCard Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			IntervalDays := 1
			input := model.NewCard{
				Front:        "New Card Front",
				Back:         "New Card Back",
				ReviewDate:   now,
				IntervalDays: &IntervalDays,
				CardgroupID:  cardGroup.ID,
				Created:      now,
				Updated:      now,
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
                "data": {
                    "createCard": {
                        "id": "1",
                        "front": "New Card Front",
                        "back": "New Card Back",
                        "interval_days": 1
                    }
                }
            }`

			testGraphQLQuery(t, e, jsonInput, expected, "data.createCard.id", "data.createCard.created", "data.createCard.updated", "data.createCard.review_date")
		})

		t.Run("UpdateCard Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			card := repository.Card{
				Front:        "Old Front",
				Back:         "Old Back",
				ReviewDate:   now,
				IntervalDays: 1,
				CardGroupID:  cardGroup.ID,
				Created:      now,
				Updated:      now,
			}
			db.Create(&card)

			input := model.NewCard{
				Front:        "Updated Front",
				Back:         "Updated Back",
				ReviewDate:   now,
				IntervalDays: func() *int { i := 3; return &i }(),
				CardgroupID:  cardGroup.ID,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewCard!) {
            updateCard(id: $id, input: $input) {
                id
                front
                back
            }
        }`,
				"variables": map[string]interface{}{
					"id":    card.ID,
					"input": input,
				},
			})

			expected := fmt.Sprintf(`{
        "data": {
            "updateCard": {
                "id": %d,
                "front": "Updated Front",
                "back": "Updated Back"
            }
        }
    }`, card.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.updateCard.created", "data.updateCard.updated", "data.updateCard.review_date", "data.updateCard.interval_days")
		})

		t.Run("DeleteCard Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			card := repository.Card{
				Front:        "Test Card Front",
				Back:         "Test Card Back",
				ReviewDate:   now,
				IntervalDays: 1,
				CardGroupID:  cardGroup.ID,
				Created:      now,
				Updated:      now,
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

		t.Run("CreateCardGroup Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			input := model.NewCardGroup{
				Name:    "New Card Group",
				CardIds: nil,
				UserIds: []int64{1, 2},
				Created: now,
				Updated: now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($input: NewCardGroup!) {
                    createCardGroup(input: $input) {
                        id
                        name
                        created
                        updated
                    }
                }`,
				"variables": map[string]interface{}{
					"input": input,
				},
			})

			expected := `{
                "data": {
                    "createCardGroup": {
                        "id": "1",
                        "name": "New Card Group"
                    }
                }
            }`

			testGraphQLQuery(t, e, jsonInput, expected, "data.createCardGroup.id", "data.createCardGroup.created", "data.createCardGroup.updated")
		})

		t.Run("UpdateCardGroup Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create an existing card group
			cardGroup := repository.Cardgroup{
				Name:    "Old Group",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&cardGroup).Error; err != nil {
				t.Fatalf("failed to create card group: %v", err)
			}

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "UpdateCardGroup Mutation Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "UpdateCardGroup Mutation Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Users Query Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign a role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			// Assign the user to the card group
			if err := db.Model(&cardGroup).Association("Users").Append(&user); err != nil {
				t.Fatalf("failed to assign user to card group: %v", err)
			}

			// Input data for updating
			input := model.NewCardGroup{
				Name:    "Updated Group",
				UserIds: []int64{user.ID},
				Created: now,
				Updated: now,
			}

			// GraphQL Mutation test for updating card group
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewCardGroup!) {
            updateCardGroup(id: $id, input: $input) {
                id
                name
                created
                updated
            }
        }`,
				"variables": map[string]interface{}{
					"id":    cardGroup.ID,
					"input": input,
				},
			})

			expected := fmt.Sprintf(`{
        "data": {
            "updateCardGroup": {
                "id": %d,
                "name": "Updated Group"
            }
        }
    }`, cardGroup.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.updateCardGroup.created", "data.updateCardGroup.updated")
		})

		t.Run("DeleteCardGroup Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			cardGroup := repository.Cardgroup{
				Name:    "Test Group",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
                    deleteCardGroup(id: $id)
                }`,
				"variables": map[string]interface{}{
					"id": cardGroup.ID,
				},
			})

			expected := `{
                "data": {
                    "deleteCardGroup": true
                }
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CreateUser Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "CreateUser Mutation Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "CreateUser Mutation Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			input := model.NewUser{
				Name:    "New User",
				RoleIds: []int64{role.ID},
				Created: now,
				Updated: now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($input: NewUser!) {
                    createUser(input: $input) {
                        id
                        name
                        created
                        updated
                    }
                }`,
				"variables": map[string]interface{}{
					"input": input,
				},
			})

			expected := `{
                "data": {
                    "createUser": {
                        "id": "1",
                        "name": "New User"
                    }
                }
            }`

			testGraphQLQuery(t, e, jsonInput, expected, "data.createUser.id", "data.createUser.created", "data.createUser.updated")
		})

		t.Run("UpdateUser Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "UpdateUser Mutation Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "UpdateUser Mutation Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Old User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign a role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			// Input data for updating
			input := model.NewUser{
				Name:    "Updated User",
				RoleIds: []int64{role.ID},
				Created: now,
				Updated: now,
			}

			// GraphQL Mutation test for updating user
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewUser!) {
            updateUser(id: $id, input: $input) {
                id
                name
                created
                updated
            }
        }`,
				"variables": map[string]interface{}{
					"id":    user.ID,
					"input": input,
				},
			})

			expected := fmt.Sprintf(`{
        "data": {
            "updateUser": {
                "id": %d,
                "name": "Updated User"
            }
        }
    }`, user.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.updateUser.created", "data.updateUser.updated")
		})

		t.Run("DeleteUser Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			user := repository.User{
				Name:    "Test User",
				Created: now,
				Updated: now,
			}
			db.Create(&user)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
                    deleteUser(id: $id)
                }`,
				"variables": map[string]interface{}{
					"id": user.ID,
				},
			})

			expected := `{
                "data": {
                    "deleteUser": true
                }
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CreateRole Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			input := model.NewRole{
				Name:    "New Role",
				Created: now,
				Updated: now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($input: NewRole!) {
                    createRole(input: $input) {
                        id
                        name
                        created
                        updated
                    }
                }`,
				"variables": map[string]interface{}{
					"input": input,
				},
			})

			expected := `{
                "data": {
                    "createRole": {
                        "id": "1",
                        "name": "New Role"
                    }
                }
            }`

			testGraphQLQuery(t, e, jsonInput, expected, "data.createRole.id", "data.createRole.created", "data.createRole.updated")
		})

		t.Run("UpdateRole Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			role := repository.Role{
				Name:    "Old Role",
				Created: now,
				Updated: now,
			}
			db.Create(&role)

			input := model.NewRole{
				Name:    "Updated Role",
				Created: now,
				Updated: now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewRole!) {
                    updateRole(id: $id, input: $input) {
                        id
                        name
                        created
                        updated
                    }
                }`,
				"variables": map[string]interface{}{
					"id":    role.ID,
					"input": input,
				},
			})

			expected := fmt.Sprintf(`{
                "data": {
                    "updateRole": {
                        "id": %d,
                        "name": "Updated Role"
                    }
                }
            }`, role.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.updateRole.created", "data.updateRole.updated")
		})

		t.Run("DeleteRole Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get a role
			var role repository.Role
			if err := db.Where("name = ?", "Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "Test Role DeleteRole",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// GraphQL Mutation test for deleting a role
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
            deleteRole(id: $id)
        }`,
				"variables": map[string]interface{}{
					"id": role.ID,
				},
			})

			expected := `{
        "data": {
            "deleteRole": true
        }
    }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("AddUserToCardGroup Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign a role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			// Create a card group
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($userID: ID!, $cardGroupID: ID!) {
                    addUserToCardGroup(userID: $userID, cardGroupID: $cardGroupID) {
                        id
                        name
                        created
                        updated
                    }
                }`,
				"variables": map[string]interface{}{
					"userID":      user.ID,
					"cardGroupID": cardGroup.ID,
				},
			})

			expected := fmt.Sprintf(`{
                "data": {
                    "addUserToCardGroup": {
                        "id": %d,
                        "name": "Test CardGroup"
                    }
                }
            }`, cardGroup.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.addUserToCardGroup.created", "data.addUserToCardGroup.updated")
		})

		t.Run("RemoveUserFromCardGroup Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "RemoveUserFromCardGroup Mutation Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "RemoveUserFromCardGroup Mutation Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign a role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			// Create a card group
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)
			db.Model(&cardGroup).Association("Users").Append(&user)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($userID: ID!, $cardGroupID: ID!) {
                    removeUserFromCardGroup(userID: $userID, cardGroupID: $cardGroupID) {
                        id
                        name
                        created
                        updated
                    }
                }`,
				"variables": map[string]interface{}{
					"userID":      user.ID,
					"cardGroupID": cardGroup.ID,
				},
			})

			expected := fmt.Sprintf(`{
                "data": {
                    "removeUserFromCardGroup": {
                        "id": %d,
                        "name": "Test CardGroup"
                    }
                }
            }`, cardGroup.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.removeUserFromCardGroup.created", "data.removeUserFromCardGroup.updated")
		})

		t.Run("AssignRoleToUser Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "AssignRoleToUser Mutation Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "AssignRoleToUser Mutation Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign the role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			// GraphQL Mutation test for assigning a role
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($userID: ID!, $roleID: ID!) {
            assignRoleToUser(userID: $userID, roleID: $roleID) {
                id
                name
                created
                updated
            }
        }`,
				"variables": map[string]interface{}{
					"userID": user.ID,
					"roleID": role.ID,
				},
			})

			expected := fmt.Sprintf(`{
        "data": {
            "assignRoleToUser": {
                "id": %d,
                "name": "Test User"
            }
        }
    }`, user.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.assignRoleToUser.created", "data.assignRoleToUser.updated")
		})

		t.Run("RemoveRoleFromUser Mutation", func(t *testing.T) {
			t.Parallel()

			now := time.Now()

			// Create or get an existing role
			var role repository.Role
			if err := db.Where("name = ?", "RemoveRoleFromUser Mutation Test Role").First(&role).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					role = repository.Role{
						Name:    "RemoveRoleFromUser Mutation Test Role",
						Created: now,
						Updated: now,
					}
					if err := db.Create(&role).Error; err != nil {
						t.Fatalf("failed to create role: %v", err)
					}
				} else {
					t.Fatalf("failed to query role: %v", err)
				}
			}

			// Create an existing user
			user := repository.User{
				Name:    "Test User",
				Created: now,
				Updated: now,
			}
			if err := db.Create(&user).Error; err != nil {
				t.Fatalf("failed to create user: %v", err)
			}

			// Assign the role to the user
			if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
				t.Fatalf("failed to assign role to user: %v", err)
			}

			// GraphQL Mutation test for removing a role
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($userID: ID!, $roleID: ID!) {
            removeRoleFromUser(userID: $userID, roleID: $roleID) {
                id
                name
                created
                updated
            }
        }`,
				"variables": map[string]interface{}{
					"userID": user.ID,
					"roleID": role.ID,
				},
			})

			expected := fmt.Sprintf(`{
        "data": {
            "removeRoleFromUser": {
                "id": %d,
                "name": "Test User"
            }
        }
    }`, user.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.removeRoleFromUser.created", "data.removeRoleFromUser.updated")
		})
	})
}

func TestGraphQLErrors(t *testing.T) {
	t.Helper()
	t.Parallel()

	testutils.RunServersTest(t, db, func(t *testing.T) {
		t.Run("Card Query with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"

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
					"id": invalidID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["card", "id"]
                }
            ],
            "data": {
                "card": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CreateCard with Missing Fields", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			cardGroup := model.CardGroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			// Missing `front` and `back` fields
			input := map[string]interface{}{
				"review_date":   now,
				"interval_days": 1,
				"cardgroup_id":  cardGroup.ID,
				"created":       now,
				"updated":       now,
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
            "errors": [
                {
                    "extensions": {
                        "code": "GRAPHQL_VALIDATION_FAILED"
                    },
                    "message": "must be defined",
                    "path": ["variable", "input", "front"]
                }
            ],
            "data": null
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UpdateCard with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"
			now := time.Now()
			input := model.NewCard{
				Front:        "Updated Front",
				Back:         "Updated Back",
				ReviewDate:   now,
				IntervalDays: func() *int { i := 3; return &i }(),
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewCard!) {
                updateCard(id: $id, input: $input) {
                    id
                    front
                    back
                }
            }`,
				"variables": map[string]interface{}{
					"id":    invalidID,
					"input": input,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["updateCard", "id"]
                }
            ],
            "data": {
                "updateCard": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("DeleteCard with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
                deleteCard(id: $id)
            }`,
				"variables": map[string]interface{}{
					"id": invalidID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["deleteCard", "id"]
                }
            ],
            "data": {
                "deleteCard": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CreateCardGroup with Missing Name", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			// Missing `name` field
			input := map[string]interface{}{
				"user_ids": []int64{1, 2},
				"created":  now,
				"updated":  now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($input: NewCardGroup!) {
                createCardGroup(input: $input) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"input": input,
				},
			})

			expected := `{
            "errors": [
                {
                    "extensions": {
                        "code": "GRAPHQL_VALIDATION_FAILED"
                    },
                    "message": "must be defined",
                    "path": ["variable", "input", "name"]
                }
            ],
            "data": null
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UpdateCardGroup with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"
			now := time.Now()
			input := model.NewCardGroup{
				Name:    "Updated Group",
				UserIds: []int64{1, 2},
				Created: now,
				Updated: now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewCardGroup!) {
                updateCardGroup(id: $id, input: $input) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"id":    invalidID,
					"input": input,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["updateCardGroup", "id"]
                }
            ],
            "data": {
                "updateCardGroup": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("DeleteCardGroup with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
                deleteCardGroup(id: $id)
            }`,
				"variables": map[string]interface{}{
					"id": invalidID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["deleteCardGroup", "id"]
                }
            ],
            "data": {
                "deleteCardGroup": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CreateUser with Missing Name", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			// Missing `name` field
			input := map[string]interface{}{
				"role_ids": []int64{1, 2},
				"created":  now,
				"updated":  now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($input: NewUser!) {
                createUser(input: $input) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"input": input,
				},
			})

			expected := `{
            "errors": [
                {
                    "extensions": {
                        "code": "GRAPHQL_VALIDATION_FAILED"
                    },
                    "message": "must be defined",
                    "path": ["variable", "input", "name"]
                }
            ],
            "data": null
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UpdateUser with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"
			now := time.Now()
			input := model.NewUser{
				Name:    "Updated User",
				RoleIds: []int64{1, 2},
				Created: now,
				Updated: now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewUser!) {
                updateUser(id: $id, input: $input) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"id":    invalidID,
					"input": input,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["updateUser", "id"]
                }
            ],
            "data": {
                "updateUser": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("DeleteUser with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
                deleteUser(id: $id)
            }`,
				"variables": map[string]interface{}{
					"id": invalidID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["deleteUser", "id"]
                }
            ],
            "data": {
                "deleteUser": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CreateRole with Missing Name", func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			// Missing `name` field
			input := map[string]interface{}{
				"created": now,
				"updated": now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($input: NewRole!) {
                createRole(input: $input) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"input": input,
				},
			})

			expected := `{
            "errors": [
                {
                    "extensions": {
                        "code": "GRAPHQL_VALIDATION_FAILED"
                    },
                    "message": "must be defined",
                    "path": ["variable", "input", "name"]
                }
            ],
            "data": null
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UpdateRole with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"
			now := time.Now()
			input := model.NewRole{
				Name:    "Updated Role",
				Created: now,
				Updated: now,
			}

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!, $input: NewRole!) {
                updateRole(id: $id, input: $input) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"id":    invalidID,
					"input": input,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["updateRole", "id"]
                }
            ],
            "data": {
                "updateRole": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("DeleteRole with Invalid ID", func(t *testing.T) {
			t.Parallel()

			invalidID := "invalid-id"

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
                deleteRole(id: $id)
            }`,
				"variables": map[string]interface{}{
					"id": invalidID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-id\": invalid syntax",
                    "path": ["deleteRole", "id"]
                }
            ],
            "data": {
                "deleteRole": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("AddUserToCardGroup with Invalid IDs", func(t *testing.T) {
			t.Parallel()

			invalidUserID := "invalid-user-id"
			invalidCardGroupID := "invalid-cardgroup-id"

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($userID: ID!, $cardGroupID: ID!) {
                addUserToCardGroup(userID: $userID, cardGroupID: $cardGroupID) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"userID":      invalidUserID,
					"cardGroupID": invalidCardGroupID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-user-id\": invalid syntax",
                    "path": ["addUserToCardGroup", "userID"]
                }
            ],
            "data": {
                "addUserToCardGroup": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("RemoveUserFromCardGroup with Invalid IDs", func(t *testing.T) {
			t.Parallel()

			invalidUserID := "invalid-user-id"
			invalidCardGroupID := "invalid-cardgroup-id"

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($userID: ID!, $cardGroupID: ID!) {
                removeUserFromCardGroup(userID: $userID, cardGroupID: $cardGroupID) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"userID":      invalidUserID,
					"cardGroupID": invalidCardGroupID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-user-id\": invalid syntax",
                    "path": ["removeUserFromCardGroup", "userID"]
                }
            ],
            "data": {
                "removeUserFromCardGroup": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("AssignRoleToUser with Invalid IDs", func(t *testing.T) {
			t.Parallel()

			invalidUserID := "invalid-user-id"
			invalidRoleID := "invalid-role-id"

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($userID: ID!, $roleID: ID!) {
                assignRoleToUser(userID: $userID, roleID: $roleID) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"userID": invalidUserID,
					"roleID": invalidRoleID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-user-id\": invalid syntax",
                    "path": ["assignRoleToUser", "userID"]
                }
            ],
            "data": {
                "assignRoleToUser": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("RemoveRoleFromUser with Invalid IDs", func(t *testing.T) {
			t.Parallel()

			invalidUserID := "invalid-user-id"
			invalidRoleID := "invalid-role-id"

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($userID: ID!, $roleID: ID!) {
                removeRoleFromUser(userID: $userID, roleID: $roleID) {
                    id
                    name
                    created
                    updated
                }
            }`,
				"variables": map[string]interface{}{
					"userID": invalidUserID,
					"roleID": invalidRoleID,
				},
			})

			expected := `{
            "errors": [
                {
                    "message": "strconv.ParseInt: parsing \"invalid-user-id\": invalid syntax",
                    "path": ["removeRoleFromUser", "userID"]
                }
            ],
            "data": {
                "removeRoleFromUser": null
            }
        }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})
	})
}
