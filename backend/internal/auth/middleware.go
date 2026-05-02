package auth

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/rotisserie/eris"

	"backend/internal/logging"
)

const wwwAuthenticate = `Bearer realm="api"`

// AuthMiddleware returns an error at construction if cfg is missing required fields,
// because jwt.WithAudience("")/WithIssuer("") would silently match tokens with empty claims.
func AuthMiddleware(kf keyfunc.Keyfunc, cfg Config) (echo.MiddlewareFunc, error) {
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
