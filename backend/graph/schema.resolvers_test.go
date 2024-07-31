package graph

import (
	"backend/graph/services"
	"backend/pkg/middlewares"
	"backend/pkg/validator"
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

	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gorilla/websocket"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	repository "backend/graph/db"
	"backend/graph/model"
	"backend/pkg/config"
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
	resolver := &Resolver{
		DB:      db,
		Srv:     service,
		VW:      validateWrapper,
		Loaders: NewLoaders(service),
	}

	srv := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolver}))

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

func TestMutationResolver(t *testing.T) {
	t.Helper()
	t.Parallel()

	testutils.RunServersTest(t, db, func(t *testing.T) {
		t.Run("CreateCard", func(t *testing.T) {
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestMutationResolver_CreateCard Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

			interval_days := 1
			input := model.NewCard{
				Front:        "CreateCard Front of card",
				Back:         "CreateCard Back of card",
				ReviewDate:   now,
				IntervalDays: &interval_days,
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
					"id": -1,
				},
			})
			expected := `{
                "data": {
                    "card": null
                },
                "errors": [{
                    "message": "invalid id: -1",
                    "path": ["card"]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UpdateCard", func(t *testing.T) {
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestMutationResolver_UpdateCard Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

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
					"id":    -1,
					"input": input,
				},
			})
			expected := `{
                "data": {
                    "updateCard": null
                },
                "errors": [{
                    "message": "invalid id: -1",
                    "path": ["updateCard"]
                }]
            }`
			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("DeleteCard", func(t *testing.T) {
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestMutationResolver_DeleteCard Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

			card := repository.Card{
				Front:        "DeleteCard Front",
				Back:         "DeleteCard Back",
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
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `mutation ($id: ID!) {
                    deleteCard(id: $id)
                }`,
				"variables": map[string]interface{}{
					"id": -1,
				},
			})
			expected := `{
                "data": {
                    "deleteCard": false
                },
                "errors": [{
                    "message": "invalid id: -1",
                    "path": ["deleteCard"]
                }]
            }`
			testGraphQLQuery(t, e, jsonInput, expected)
		})
	})
}

func TestQueryResolver(t *testing.T) {
	t.Helper()
	t.Parallel()

	testutils.RunServersTest(t, db, func(t *testing.T) {
		t.Run("CardsByCardGroup", func(t *testing.T) {
			now := time.Now().UTC()
			cardGroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardGroup)

			card := repository.Card{
				Front:        "Card Front",
				Back:         "Card Back",
				ReviewDate:   now,
				IntervalDays: 1,
				CardGroupID:  cardGroup.ID,
				Created:      now,
				Updated:      now,
			}
			db.Create(&card)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($cardGroupID: ID!) {
					cardsByCardGroup(cardGroupID: $cardGroupID) {
						edges {
							node {
								id
								front
								back
								review_date
								interval_days
								created
								updated
							}
						}
					}
				}`,
				"variables": map[string]interface{}{
					"cardGroupID": cardGroup.ID,
				},
			})

			expected := fmt.Sprintf(`{
				"data": {
					"cardsByCardGroup": {
						"edges": [{
							"node": {
								"id": %d,
								"front": "Card Front",
								"back": "Card Back",
								"interval_days": 1
							}
						}]
					}
				}
			}`, card.ID)

			testGraphQLQuery(t, e, jsonInput, expected, "data.cardsByCardGroup.edges.node.created", "data.cardsByCardGroup.edges.node.updated", "data.cardsByCardGroup.edges.node.review_date")
		})

		t.Run("Card", func(t *testing.T) {
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "TestQueryResolver_Card Cardgroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

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
                        "back": "Card Back",
                        "review_date": "%s",
                        "interval_days": 1,
                        "created": "%s",
                        "updated": "%s"
                    }
                }
            }`, card.ID, card.ReviewDate.Format(time.RFC3339Nano), card.Created.Format(time.RFC3339Nano), card.Updated.Format(time.RFC3339Nano))

			testGraphQLQuery(t, e, jsonInput, expected, "data.card.id", "data.card.created", "data.card.updated", "data.card.review_date")
		})

		t.Run("Card_Error", func(t *testing.T) {
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
					"id": -1,
				},
			})
			expected := `{
                "data": {
                    "card": null
                },
                "errors": [{
                    "message": "invalid id: -1",
                    "path": ["card"]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("Roles", func(t *testing.T) {
			role := repository.Role{
				Name: "Test Role",
			}
			db.Create(&role)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
                    roles {
                        id
                        name
                    }
                }`,
			})

			req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBuffer(jsonInput))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)

			var response map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			roles, ok := response["data"].(map[string]interface{})["roles"].([]interface{})
			if !ok {
				t.Fatalf("Failed to parse roles from response")
			}
			assert.Len(t, roles, 1, "Expected number of roles to be 1")

			roleDetails := roles[0].(map[string]interface{})
			assert.Equal(t, "Test Role", roleDetails["name"])
		})

		t.Run("Roles_Error", func(t *testing.T) {
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
                    roles {
                        invalid_field
                    }
                }`,
			})
			expected := `{
                "data": null,
                "errors": [{
                    "message": "Cannot query field \"invalid_field\" on type \"Role\".",
                    "extensions": {
                        "code": "GRAPHQL_VALIDATION_FAILED"
                    },
                    "locations": [{
                        "line": 3,
                        "column": 17
                    }]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("Role", func(t *testing.T) {
			if err := db.Where("name = ?", "Test Role").Delete(&repository.Role{}).Error; err != nil {
				t.Fatalf("Failed to delete existing Test Role: %v", err)
			}

			role := repository.Role{
				Name: "Test Role",
			}
			db.Create(&role)

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
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
                    role(id: $id) {
                        id
                        name
                    }
                }`,
				"variables": map[string]interface{}{
					"id": -1,
				},
			})
			expected := `{
                "data": {
                    "role": null
                },
                "errors": [{
                    "message": "invalid id: -1",
                    "path": ["role"]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("Users", func(t *testing.T) {
			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)

			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
                    users {
                        id
                        name
                    }
                }`,
			})

			req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBuffer(jsonInput))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)

			var response map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatalf("Failed to unmarshal response: %v", err)
			}

			users, ok := response["data"].(map[string]interface{})["users"].([]interface{})
			if !ok {
				t.Fatalf("Failed to parse users from response")
			}
			assert.Len(t, users, 1, "Expected number of users to be 1")

			userDetails := users[0].(map[string]interface{})
			assert.Equal(t, "Test User", userDetails["name"])
		})

		t.Run("Users_Error", func(t *testing.T) {
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `{
                    users {
                        invalid_field
                    }
                }`,
			})
			expected := `{
                "errors": [{
                    "message": "Cannot query field \"invalid_field\" on type \"User\".",
                    "locations": [{"line": 3, "column": 5}],
                    "extensions": {
                        "code": "GRAPHQL_VALIDATION_FAILED"
                    }
                }],
                "data": null
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("User", func(t *testing.T) {
			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)

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
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($id: ID!) {
                    user(id: $id) {
                        id
                        name
                    }
                }`,
				"variables": map[string]interface{}{
					"id": -1,
				},
			})
			expected := `{
                "data": {
                    "user": null
                },
                "errors": [{
                    "message": "invalid id: -1",
                    "path": ["user"]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CardsByCardGroup", func(t *testing.T) {
			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)

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
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($cardGroupId: ID!) {
                    cardsByCardGroup(cardGroupId: $cardGroupId) {
                        id
                        front
                        back
                    }
                }`,
				"variables": map[string]interface{}{
					"cardGroupId": -1,
				},
			})
			expected := `{
                "data": null,
                "errors": [{
                    "message": "invalid cardGroupID: -1",
                    "path": ["cardsByCardGroup"]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UserRole", func(t *testing.T) {
			var role repository.Role
			if err := db.Where("name = ?", "Test Role").First(&role).Error; err != nil {
				role = repository.Role{
					Name: "Test Role",
				}
				db.Create(&role)
			}

			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)
			db.Model(&user).Association("Roles").Append(&role)

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
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($userId: ID!) {
                    userRole(userId: $userId) {
                        id
                        name
                    }
                }`,
				"variables": map[string]interface{}{
					"userId": -1,
				},
			})
			expected := `{
                "data": {
                    "userRole": null
                },
                "errors": [{
                    "message": "invalid userID: -1",
                    "path": ["userRole"]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("CardGroupsByUser", func(t *testing.T) {
			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)

			now := time.Now()
			cardgroup := repository.Cardgroup{
				Name:    "Test CardGroup",
				Created: now,
				Updated: now,
			}
			db.Create(&cardgroup)
			db.Model(&cardgroup).Association("Users").Append(&user)

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
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($userId: ID!) {
                    cardGroupsByUser(userId: $userId) {
                        id
                        name
                    }
                }`,
				"variables": map[string]interface{}{
					"userId": -1,
				},
			})
			expected := `{
                "data": null,
                "errors": [{
                    "message": "invalid userID: -1",
                    "path": ["cardGroupsByUser"]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})

		t.Run("UsersByRole", func(t *testing.T) {
			var existingRole repository.Role
			roleName := "Test Role"

			if err := db.Where("name = ?", roleName).First(&existingRole).Error; err == nil {
				db.Delete(&existingRole)
			}

			role := repository.Role{
				Name: roleName,
			}
			db.Create(&role)

			user := repository.User{
				Name: "Test User",
			}
			db.Create(&user)
			db.Model(&user).Association("Roles").Append(&role)

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
			jsonInput, _ := json.Marshal(map[string]interface{}{
				"query": `query ($roleId: ID!) {
                    usersByRole(roleId: $roleId) {
                        id
                        name
                    }
                }`,
				"variables": map[string]interface{}{
					"roleId": -1,
				},
			})
			expected := `{
                "data": null,
                "errors": [{
                    "message": "invalid roleID: -1",
                    "path": ["usersByRole"]
                }]
            }`

			testGraphQLQuery(t, e, jsonInput, expected)
		})
	})
}
