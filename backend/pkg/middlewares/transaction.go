package middlewares

import (
	"backend/pkg/logger"
	"bytes"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/context"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strings"
)

var DbContext = "db"
var TxContext = "tx"

type GraphQLRequest struct {
	Query string `json:"query"`
}

func DatabaseCtxMiddleware(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(DbContext, db)
			return next(c)
		}
	}
}

func TransactionMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get the standard context from echo.Context
			ctx := c.Request().Context()

			// Check if the request is POST and the path is /query
			if c.Request().Method != http.MethodPost || c.Path() != "/query" {
				return next(c)
			}

			db, ok := c.Get(DbContext).(*gorm.DB)
			if !ok {
				logger.Logger.ErrorContext(ctx, "Database connection not found")
				return echo.NewHTTPError(http.StatusInternalServerError, "Database connection not found")
			}

			// Read the body
			bodyBytes, err := io.ReadAll(c.Request().Body)
			if err != nil {
				logger.Logger.ErrorContext(ctx, "Failed to read request body: %v", err)
				return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload")
			}

			// Restore the body so it can be read again
			c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			var gqlReq GraphQLRequest
			if err := json.Unmarshal(bodyBytes, &gqlReq); err != nil {
				logger.Logger.ErrorContext(ctx, "Failed to unmarshal request body: %v", err)
				return echo.NewHTTPError(http.StatusBadRequest, "Invalid request payload")
			}

			// Check if the operation is a mutation
			if strings.HasPrefix(strings.TrimSpace(gqlReq.Query), "mutation") {
				tx := db.Begin()
				if tx.Error != nil {
					logger.Logger.ErrorContext(ctx, "Failed to start transaction: %v", tx.Error)
					return echo.NewHTTPError(http.StatusInternalServerError, "Failed to start transaction")
				}

				defer func() {
					if r := recover(); r != nil {
						tx.Rollback()
						panic(r)
					} else if c.Response().Status >= http.StatusBadRequest {
						if err := tx.Rollback().Error; err != nil {
							logger.Logger.ErrorContext(ctx, "Failed to rollback transaction: %v", err)
						}
					} else {
						if err := tx.Commit().Error; err != nil {
							logger.Logger.ErrorContext(ctx, "Failed to commit transaction: %v", err)
						}
					}
				}()

				ctx := context.WithValue(c.Request().Context(), TxContext, tx)
				c.SetRequest(c.Request().WithContext(ctx))
			}

			return next(c)
		}
	}
}

// GetDBFromContext retrieves the GORM DB transaction from the context
func GetDBFromContext(ctx context.Context) *gorm.DB {
	tx, _ := ctx.Value(TxContext).(*gorm.DB)
	if tx != nil {
		return tx
	}
	// Fallback to the main DB connection if no transaction is found
	return ctx.Value(DbContext).(*gorm.DB)
}
