package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

const wwwAuthenticate = `Bearer realm="api"`

// AuthMiddleware verifies a Supabase-issued JWT from the Authorization header.
// No header → request proceeds as anonymous (ctx has no AuthUser).
// Header present + verification succeeds → AuthUser attached to ctx.
// Header present + verification fails → 401 with WWW-Authenticate: Bearer realm="api".
func AuthMiddleware(kf keyfunc.Keyfunc, cfg Config) echo.MiddlewareFunc {
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
				Sub:   claims.Subject,
				Email: claims.Email,
				Role:  claims.Role,
			}
			r := c.Request()
			c.SetRequest(r.WithContext(withUser(r.Context(), u)))
			return next(c)
		}
	}
}

func extractBearer(header string) (string, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("auth: Authorization header must use Bearer scheme")
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("auth: empty bearer token")
	}
	return token, nil
}

func reject(c *echo.Context, cause error) error {
	slog.Warn("auth: token rejected", "err", cause, "path", c.Request().URL.Path)
	c.Response().Header().Set(echo.HeaderWWWAuthenticate, wwwAuthenticate)
	return c.String(http.StatusUnauthorized, "invalid token")
}
