package auth

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/labstack/echo/v5"
	"github.com/rotisserie/eris"

	"backend/internal/logging"
)

// adminChecker reports whether a user already holds the admin role.
// Satisfied by *auth.Service.
type adminChecker interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

// RoleAssigner is the narrow role-assignment port required by SuperUserPromoter.
// It is exported so the composition root can widen concrete repositories
// before injecting them, preserving the intended component boundary.
type RoleAssigner interface {
	AssignRoleToUser(ctx context.Context, userID, roleID string) error
}

// SuperUserPromoter is an Echo middleware factory that grants the admin role
// to first-time logins from a configured set of trusted email addresses.
type SuperUserPromoter struct {
	emails      map[string]struct{} // canonicalised
	adminRoleID string
	checker     adminChecker
	assigner    RoleAssigner
	logger      *slog.Logger

	// confirmedAdmins is the process-lifetime set of subs already known to hold the
	// admin role. Once a sub is recorded, the middleware skips the per-request
	// IsAdmin role query for it. A sub is recorded only by a request from that same
	// sub, so an account that has not been seen since the process started is absent
	// from the set regardless of how long the process has been running.
	//
	// The admin role CAN be removed while the process runs: adminEditUser replaces
	// a user's final role set and adminDeleteUser removes the account outright, so
	// a recorded sub may no longer be an admin. A cache with no TTL/eviction is
	// nonetheless safe, because this cache gates only *re-promotion* — it is never
	// consulted for authorization. Every admin-gated operation goes through
	// usecase.AdminGate.Require, which issues a fresh IsAdmin database read on
	// every call and returns a forbidden error when it comes back false. A stale
	// entry can therefore only cause a missed re-promotion, never a privilege leak.
	//
	// SUPER_USER_EMAILS membership cannot change without a process restart. A
	// restart also empties the cache, so re-promotion becomes certain for every
	// still-listed address at that point — but an uncached sub is re-promoted on
	// its next request without one. The zero value is ready to use.
	confirmedAdmins sync.Map // map[string]struct{}
}

// ParseSuperUserSet splits a comma-separated string of email addresses into a
// canonicalised, deduplicated set. Returns an empty (non-nil) map when raw is blank.
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

// canonicalEmail normalizes an email address for comparison.
func canonicalEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// NewSuperUserPromoter constructs a promoter. When emails is empty, Middleware
// returns a zero-cost pass-through; checker, assigner, and adminRoleID may be
// nil/empty in that case. Panics if emails is non-empty but any dependency is
// missing — catches misconfiguration at process startup, not per-request.
//
// logger is the injected structured logger the middleware uses for its WARN and
// INFO promotion events. A nil logger falls back to slog.Default() so existing
// callers that do not yet thread a logger keep working; production callers
// should pass the composition-root logger so the events carry request context.
func NewSuperUserPromoter(
	emails map[string]struct{},
	adminRoleID string,
	checker adminChecker,
	assigner RoleAssigner,
	logger *slog.Logger,
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

	if logger == nil {
		logger = slog.Default()
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
		logger:      logger,
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

			// Already confirmed as admin in this process: skip the role query.
			// Skipping it can only miss a re-promotion, never grant access — every
			// admin-gated operation re-reads the role (see the confirmedAdmins field).
			if _, ok := p.confirmedAdmins.Load(u.Sub); ok {
				return next(c)
			}

			isAdmin, err := p.checker.IsAdmin(ctx, u.Sub)
			if err != nil {
				logging.LogWarn(ctx, p.logger, "superuser: admin check failed",
					eris.Wrap(err, "superuser: IsAdmin"),
					slog.String("user_id", u.Sub))
				return next(c)
			}
			if isAdmin {
				// Hot-path short-circuit: already has the role, nothing to do.
				// Record it so subsequent requests skip the IsAdmin query.
				p.confirmedAdmins.Store(u.Sub, struct{}{})
				return next(c)
			}

			if err := p.assigner.AssignRoleToUser(ctx, u.Sub, p.adminRoleID); err != nil {
				logging.LogWarn(ctx, p.logger, "superuser: role assignment failed",
					eris.Wrap(err, "superuser: AssignRoleToUser"),
					slog.String("user_id", u.Sub))
				return next(c)
			}

			// Promotion succeeded: record the sub so future requests skip the query.
			p.confirmedAdmins.Store(u.Sub, struct{}{})
			p.logger.InfoContext(ctx, "superuser: promoted to admin", slog.String("user_id", u.Sub))
			return next(c)
		}
	}
}
