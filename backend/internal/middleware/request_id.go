package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

const RequestIDHeader = "X-Request-ID"

// maxRequestIDLen is the maximum accepted length for an incoming X-Request-ID value.
// Values longer than this are dropped and regenerated to prevent log injection.
const maxRequestIDLen = 128

type requestIDKey struct{}

// RequestIDFromContext returns the request ID stored in ctx, or "" if none is present.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}

// WithRequestID returns a child of ctx with the given request ID stored under
// the same key that RequestIDFromContext reads. Use this to propagate a
// request-scoped ID into a fresh context (e.g. a fire-and-forget goroutine
// that cannot borrow the request context because it outlives the request).
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func generateRequestID() string {
	id, err := uuid.NewV7()
	if err != nil {
		slog.Error("request_id: uuid.NewV7 failed; using timestamp fallback", "err", err)
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return id.String()
}

// RequestID returns an Echo middleware that reads X-Request-ID from the incoming
// request, stores it in the request context, and echoes it in the response header.
// Absent or overlong headers are replaced with a generated UUIDv7.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			incoming := c.Request().Header.Get(RequestIDHeader)

			var id string
			rejected := incoming != "" && len(incoming) > maxRequestIDLen
			if incoming != "" && !rejected {
				id = incoming
			} else {
				id = generateRequestID()
			}

			ctx := context.WithValue(c.Request().Context(), requestIDKey{}, id)
			c.SetRequest(c.Request().WithContext(ctx))

			// Warn after enriching the context so ContextHandler attaches request_id.
			if rejected {
				slog.WarnContext(ctx,
					"request_id: rejected incoming X-Request-ID, regenerating",
					"incoming_len", len(incoming))
			}

			// Set response header before next so error responses also carry the ID.
			c.Response().Header().Set(RequestIDHeader, id)

			return next(c)
		}
	}
}
