package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/rotisserie/eris"

	"backend/internal/logging"
	internalmw "backend/internal/middleware"
)

const wwwAuthenticate = `Bearer realm="api"`

// lastActiveToucher is the subset of repository.UserRepository used by
// AuthMiddleware to record authenticated activity. The narrow interface keeps
// the middleware decoupled from the full repository surface and simplifies
// test doubles.
type lastActiveToucher interface {
	TouchLastActive(ctx context.Context, userID string) error
}

// AuthMiddleware returns an error at construction if cfg is missing required
// fields, because jwt.WithAudience("")/WithIssuer("") would silently match
// tokens with empty claims.
//
// repo is optional. When provided, a fire-and-forget goroutine writes
// last_active = NOW() for every successfully authenticated request. Pass nil
// (or omit) to disable the behaviour (e.g. in unit tests that do not need a
// database).
func AuthMiddleware(kf keyfunc.Keyfunc, cfg Config, repo lastActiveToucher) (echo.MiddlewareFunc, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"ES256", "RS256"}),
		jwt.WithAudience(cfg.Audience),
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			raw := c.Request().Header.Get("Authorization")
			if raw == "" {
				return next(c)
			}

			tokenStr, err := extractBearer(raw)
			if err != nil {
				return reject(c, err)
			}

			var claims supabaseClaims
			if _, err := parser.ParseWithClaims(tokenStr, &claims, kf.Keyfunc); err != nil {
				return reject(c, err)
			}

			u := &AuthUser{
				Sub:           claims.Subject,
				Email:         claims.Email,
				EmailVerified: claims.EmailVerified,
				Role:          claims.Role,
			}
			r := c.Request()
			c.SetRequest(r.WithContext(withUser(r.Context(), u)))

			// Fire-and-forget: update last_active asynchronously so the DB
			// round-trip does not add latency to the hot request path. A lost
			// update is acceptable because last_active is for human display only
			// and does not affect auth correctness.
			if repo != nil {
				// Read request_id before the goroutine starts: the request context
				// is cancelled when the response is flushed, but the ID string is
				// cheap to copy and we want the WARN log to carry the originating
				// request's ID so operators can correlate by grep.
				reqID := internalmw.RequestIDFromContext(r.Context())
				go func(userID, requestID string) {
					bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					// Re-attach request_id to the fresh background context so the
					// ContextHandler slog handler includes it in the WARN log line.
					if requestID != "" {
						bgCtx = internalmw.WithRequestID(bgCtx, requestID)
					}
					if err := repo.TouchLastActive(bgCtx, userID); err != nil {
						logging.LogWarn(bgCtx, slog.Default(), "auth: last_active update failed",
							eris.Wrap(err, "auth: TouchLastActive"),
							slog.String("user_id", userID))
					}
				}(u.Sub, reqID)
			}

			return next(c)
		}
	}, nil
}

func extractBearer(header string) (string, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", eris.New("auth: Authorization header must use Bearer scheme")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", eris.New("auth: empty bearer token")
	}
	return token, nil
}

func reject(c *echo.Context, cause error) error {
	logging.LogWarn(c.Request().Context(), slog.Default(), "auth: token rejected", cause,
		slog.String("path", c.Request().URL.Path),
		slog.String("remote_addr", c.Request().RemoteAddr),
	)
	c.Response().Header().Set(echo.HeaderWWWAuthenticate, wwwAuthenticate)
	return c.String(http.StatusUnauthorized, "invalid token")
}
