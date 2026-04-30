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

// New builds a Handler. Panics if token is empty.
func New(repo repository.PingRecordRepository, token string) *Handler {
	if token == "" {
		panic("ping.New: token must not be empty")
	}
	return &Handler{repo: repo, token: token}
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
		slog.ErrorContext(ctx, "ping: count failed", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}

	if n == 0 {
		if err := h.repo.Create(ctx); err != nil {
			slog.ErrorContext(ctx, "ping: create failed", "err", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return c.JSON(http.StatusOK, map[string]any{"action": "created", "count": 1})
	}

	deleted, err := h.repo.DeleteAll(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "ping: delete failed", "err", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal server error"})
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
			return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
		},
		DenyHandler: func(c *echo.Context, identifier string, err error) error {
			slog.WarnContext(c.Request().Context(), "ping: rate limit exceeded", "ip", identifier)
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		},
	})
}
