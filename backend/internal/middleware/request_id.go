// Package middleware provides Echo middleware helpers for the flamingo-armond backend.
package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// RequestIDHeader is the canonical HTTP header name for the request identifier.
const RequestIDHeader = "X-Request-ID"

// maxRequestIDLen is the maximum accepted length for an incoming X-Request-ID value.
// Values longer than this are dropped and regenerated to prevent log injection.
const maxRequestIDLen = 128

// requestIDKey is the unexported context key type for the request ID.
type requestIDKey struct{}

// RequestIDFromContext returns the request ID stored in ctx, or "" if none is present.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}

// contextWithRequestID stores id in ctx under the requestIDKey.
func contextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// generateRequestID creates a new UUIDv7 string. On the rare event that
// uuid.NewV7 fails (e.g. rand source unavailable), it falls back to UUIDv4.
func generateRequestID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}
	return id.String()
}

// RequestID returns an Echo middleware that reads X-Request-ID from the incoming
// request and stores it in the request context. If the header is absent or its
// value exceeds maxRequestIDLen, a new UUIDv7 is generated instead.
//
// The resolved ID is echoed in the X-Request-ID response header before the
// handler chain runs so that error responses also carry the header.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			incoming := c.Request().Header.Get(RequestIDHeader)

			var id string
			if incoming != "" && len(incoming) <= maxRequestIDLen {
				// Honour a well-formed upstream ID (e.g. cloud LB, gateway).
				id = incoming
			} else {
				id = generateRequestID()
			}

			// Set the response header before calling next so that even error
			// responses produced by downstream handlers carry the request ID.
			c.Response().Header().Set(RequestIDHeader, id)

			// Stash the ID in the request context so all downstream code
			// (resolvers, slog handler, etc.) can read it without touching HTTP.
			ctx := contextWithRequestID(c.Request().Context(), id)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}
