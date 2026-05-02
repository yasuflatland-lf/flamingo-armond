package auth

import (
	"context"
	"log/slog"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/rotisserie/eris"

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

// canonicalEmail centralizes email normalization to prevent case/whitespace
// mismatches when comparing AuthUser.Email against the configured set.
func canonicalEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// NewSuperUserPromoter constructs a promoter.
// When emails is empty (len == 0), Middleware() returns a pass-through closure
// with zero per-request cost. It is safe to pass nil for checker, assigner,
// and an empty adminRoleID when emails is empty.
// Panics at startup if emails is non-empty but checker, assigner, or adminRoleID
// are missing — these are programmer errors caught at process initialization.
func NewSuperUserPromoter(
	emails map[string]struct{},
	adminRoleID string,
	checker adminChecker,
	assigner roleAssigner,
) *SuperUserPromoter {
	if len(emails) > 0 {
		if checker == nil {
			panic("auth: NewSuperUserPromoter: checker must not be nil when emails is non-empty")
		}
		if assigner == nil {
			panic("auth: NewSuperUserPromoter: assigner must not be nil when emails is non-empty")
		}
		if adminRoleID == "" {
			panic("auth: NewSuperUserPromoter: adminRoleID must not be empty when emails is non-empty")
		}
	}

	emailsCopy := make(map[string]struct{}, len(emails))
	for k := range emails {
		emailsCopy[k] = struct{}{}
	}

	return &SuperUserPromoter{
		emails:      emailsCopy,
		adminRoleID: adminRoleID,
		checker:     checker,
		assigner:    assigner,
	}
}

// Middleware returns an Echo middleware. Place it after AuthMiddleware and
// before loader.Middleware on the /query group.
//
// When the promoter was constructed with an empty email set, the returned
// middleware is a zero-branch pass-through with no per-request overhead.
func (p *SuperUserPromoter) Middleware() echo.MiddlewareFunc {
	if len(p.emails) == 0 {
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
				// Unverified emails are not promoted: Supabase only sets email_verified
				// after the user confirms ownership, and granting admin to an unconfirmed
				// account would expose a hijack vector.
				return next(c)
			}
			if _, ok := p.emails[canonicalEmail(u.Email)]; !ok {
				return next(c)
			}

			isAdmin, err := p.checker.IsAdmin(ctx, u.Sub)
			if err != nil {
				logging.LogWarn(ctx, slog.Default(), "superuser: admin check failed",
					eris.Wrap(err, "superuser: IsAdmin"),
					slog.String("user_id", u.Sub))
				return next(c)
			}
			if isAdmin {
				// Hot-path short-circuit: already has the role, nothing to do.
				return next(c)
			}

			if err := p.assigner.AssignToUser(ctx, u.Sub, p.adminRoleID); err != nil {
				logging.LogWarn(ctx, slog.Default(), "superuser: role assignment failed",
					eris.Wrap(err, "superuser: AssignToUser"),
					slog.String("user_id", u.Sub))
				return next(c)
			}

			slog.InfoContext(ctx, "superuser: promoted to admin", slog.String("user_id", u.Sub))
			return next(c)
		}
	}
}
