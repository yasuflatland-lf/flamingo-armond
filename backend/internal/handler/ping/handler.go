package ping

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"golang.org/x/time/rate"

	"backend/internal/repository"
)

// Handler holds dependencies for the /internal/ping endpoint.
type Handler struct {
	repo  repository.PingRecordRepository
	token string
}

// New builds a Handler. The panic on empty token is a programmer-error guard
// (defense in depth); production startup is gated earlier by the PING_TOKEN
// fail-fast in run().
func New(repo repository.PingRecordRepository, token string) *Handler {
	if token == "" {
		panic("ping.New: token must not be empty")
	}
	return &Handler{repo: repo, token: token}
}

// internalError logs the given message and returns a sanitized 500 response.
// Using a helper de-duplicates the three identical error-response sites in Handle.
func internalError(c *echo.Context, msg string, err error) error {
	slog.ErrorContext(c.Request().Context(), msg, "err", err)
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

// Handle is the echo.HandlerFunc for POST /internal/ping.
// Behavior:
//   - 401 + {"error":"unauthorized"} on missing/invalid bearer token (constant-time compare).
//   - On count==0: Create → 200 {"action":"created","count":1}.
//   - On count>0:  DeleteAll → 200 {"action":"deleted","count":<rowsAffected>}.
//   - On any repo error: 500 + {"error":"internal server error"}.
func (h *Handler) Handle(c *echo.Context) error {
	tokenBytes := []byte(h.token)
	authHeader := c.Request().Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	// Constant-time compare to prevent timing oracle attacks.
	if subtle.ConstantTimeCompare([]byte(token), tokenBytes) != 1 {
		slog.WarnContext(c.Request().Context(), "ping: unauthorized",
			"remote_ip", c.RealIP(),
		)
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	ctx := c.Request().Context()
	n, err := h.repo.Count(ctx)
	if err != nil {
		return internalError(c, "ping: count failed", err)
	}

	if n == 0 {
		if err := h.repo.Create(ctx); err != nil {
			return internalError(c, "ping: create failed", err)
		}
		return c.JSON(http.StatusOK, map[string]any{"action": "created", "count": 1})
	}

	deleted, err := h.repo.DeleteAll(ctx)
	if err != nil {
		return internalError(c, "ping: delete failed", err)
	}
	return c.JSON(http.StatusOK, map[string]any{"action": "deleted", "count": deleted})
}

// RateLimiter returns the per-IP rate limiter middleware configured for this endpoint.
// Config: Rate=1 req/s, Burst=5, ExpiresIn=3m, key=c.RealIP().
func (h *Handler) RateLimiter() echo.MiddlewareFunc {
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      float64(rate.Limit(1)),
				Burst:     5,
				ExpiresIn: 3 * time.Minute,
			},
		),
		IdentifierExtractor: func(c *echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		ErrorHandler: func(c *echo.Context, err error) error {
			slog.WarnContext(c.Request().Context(), "ping: rate limiter extractor error", "err", err)
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		},
		DenyHandler: func(c *echo.Context, identifier string, err error) error {
			slog.WarnContext(c.Request().Context(), "ping: rate limit exceeded", "ip", identifier)
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		},
	})
}
