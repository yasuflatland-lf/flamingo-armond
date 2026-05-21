package usecase

import (
	"context"
	"errors"

	"backend/internal/auth"
	"backend/internal/usecase/ucerr"

	"github.com/rotisserie/eris"
)

// AdminChecker is the auth-service surface that AdminGate (and, for the
// non-gate caller in user.go, UserUsecase) consumes to determine whether
// the bearer holds the admin role. Implemented by *auth.Service in
// production; the local interface keeps the usecase decoupled from the
// auth package's concrete struct so tests can substitute a stub.
type AdminChecker interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

// AdminGate consolidates admin-authorization for usecase entry points. A
// single instance is constructed at composition time (see cmd/server/main.go)
// and injected into every admin-gated usecase via its constructor.
type AdminGate struct {
	checker AdminChecker
}

// NewAdminGate constructs an AdminGate. Panics when checker is nil — the
// invariant is enforced at construction time rather than checked on every
// Require call. The same shape is used by every other usecase constructor
// in this package for required deps (cf. NewLearnUsecase, NewSwipeUsecase).
func NewAdminGate(checker AdminChecker) *AdminGate {
	if checker == nil {
		panic("usecase: admin gate: checker must not be nil")
	}
	return &AdminGate{checker: checker}
}

// Require returns the caller's user ID after confirming the bearer is an
// admin. Return paths:
//
//   - ucerr.ErrUnauthenticated — no caller on the context (caller == nil
//     or empty Sub), or pass-through if IsAdmin itself returns it
//     (defensive; current auth.Service.IsAdmin does not).
//   - context.Canceled / context.DeadlineExceeded — propagated from the
//     IsAdmin call, unwrapped (so the resolver matches via errors.Is).
//   - *ucerr.ForbiddenError — pass-through if IsAdmin returns it
//     (defensive); also returned directly with "admin only" when the
//     check completes and the bearer is not an admin.
//   - eris.Wrap(err, callerPrefix) — every other IsAdmin error gets the
//     caller-supplied per-module prefix. The wrap is intentionally
//     applied here so callers no longer need a paired classifier helper.
//     callerPrefix MUST be non-empty (e.g. "usecase: admin role: check
//     admin"); an empty prefix produces a malformed error_chain entry.
//   - nil — the bearer is an admin; callerID == caller.Sub.
//
// Sentinels and typed errors that propagate from IsAdmin are returned
// without applying callerPrefix — the resolver layer matches them via
// errors.Is / errors.AsType and an additional wrap would defeat that.
func (g *AdminGate) Require(ctx context.Context, callerPrefix string) (callerID string, err error) {
	caller := auth.UserFrom(ctx)
	if caller == nil || caller.Sub == "" {
		return "", ucerr.ErrUnauthenticated
	}
	isAdmin, err := g.checker.IsAdmin(ctx, caller.Sub)
	if err != nil {
		if isContextDone(err) || errors.Is(err, ucerr.ErrUnauthenticated) {
			return "", err
		}
		if _, ok := errors.AsType[*ucerr.ForbiddenError](err); ok {
			return "", err
		}
		return "", eris.Wrap(err, callerPrefix)
	}
	if !isAdmin {
		return "", ucerr.NewForbiddenError("admin only")
	}
	return caller.Sub, nil
}
