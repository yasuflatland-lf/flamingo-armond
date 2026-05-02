package auth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/labstack/echo/v5"

	"backend/internal/logging"
)

// adminChecker reports whether a user already holds the admin role.
// Satisfied by *auth.Service.
type adminChecker interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

// roleAssigner grants a role to a user. Satisfied by repository.RoleRepository.
// Idempotent via ON CONFLICT DO NOTHING.
type roleAssigner interface {
	AssignToUser(ctx context.Context, userID, roleID string) error
}

// SuperUserPromoter is an Echo middleware factory that grants the admin role
// to first-time logins from a configured set of trusted email addresses.
type SuperUserPromoter struct {
	emails      map[string]struct{} // canonicalised
	adminRoleID string
	checker     adminChecker
	assigner    roleAssigner
	// passthrough is set at construction time when emails is empty, so that
	// Middleware() returns a no-op closure with zero per-request branches.
	passthrough bool
}

// ParseSuperUserSet splits a comma-separated string of email addresses into a
// canonicalised, deduplicated set. Returns an empty (non-nil) map when raw is
// blank. Exported so main.go can call it outside the package.
func ParseSuperUserSet(raw string) map[string]struct{} {
	out := make(map[string]struct{})
	if raw == "" {
		return out
	}
	for _, part := range strings.Split(raw, ",") {
		email := canonicalEmail(part)
		if email == "" {
			continue
		}
		out[email] = struct{}{}
	}
	return out
}

// canonicalEmail lowercases and trims s. Used both when parsing the env var
// and when comparing against AuthUser.Email — defence-in-depth in one place.
func canonicalEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// NewSuperUserPromoter constructs a promoter.
// When emails is empty (len == 0), Middleware() returns a pass-through closure
// decided at construction time (zero per-request cost when the feature is
// disabled). It is safe to pass nil for checker and assigner when emails is
// empty.
func NewSuperUserPromoter(
	emails map[string]struct{},
	adminRoleID string,
	checker adminChecker,
	assigner roleAssigner,
) *SuperUserPromoter {
	p := &SuperUserPromoter{
		emails:      emails,
		adminRoleID: adminRoleID,
		checker:     checker,
		assigner:    assigner,
	}
	if len(emails) == 0 {
		p.passthrough = true
	}
	return p
}

// Middleware returns an Echo middleware. Place it after AuthMiddleware and
// before loader.Middleware on the /query group.
//
// When the promoter was constructed with an empty email set, the returned
// middleware is a zero-branch pass-through decided at construction time.
func (p *SuperUserPromoter) Middleware() echo.MiddlewareFunc {
	if p.passthrough {
		return func(next echo.HandlerFunc) echo.HandlerFunc {
			return next
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			ctx := c.Request().Context()

			u := UserFrom(ctx)
			if u == nil || u.Sub == "" {
				return next(c)
			}
			if !u.EmailVerified {
				// Q5=B security gate: unverified emails are not promoted.
				return next(c)
			}
			if _, ok := p.emails[canonicalEmail(u.Email)]; !ok {
				return next(c)
			}

			isAdmin, err := p.checker.IsAdmin(ctx, u.Sub)
			if err != nil {
				logging.LogWarn(ctx, slog.Default(), "superuser: admin check failed", err,
					slog.String("user_id", u.Sub))
				return next(c)
			}
			if isAdmin {
				// Hot-path short-circuit: already has the role, nothing to do.
				return next(c)
			}

			if err := p.assigner.AssignToUser(ctx, u.Sub, p.adminRoleID); err != nil {
				logging.LogWarn(ctx, slog.Default(), "superuser: role assignment failed", err,
					slog.String("user_id", u.Sub))
				return next(c)
			}

			slog.InfoContext(ctx, "superuser: promoted to admin", slog.String("user_id", u.Sub))
			return next(c)
		}
	}
}
