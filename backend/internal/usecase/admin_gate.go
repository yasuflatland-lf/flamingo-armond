package usecase

import (
	"context"
	"errors"

	"backend/internal/auth"
	"backend/internal/usecase/ucerr"

	"github.com/rotisserie/eris"
)

// AdminChecker is the auth-service surface every admin-gated usecase needs.
// Implemented by *auth.Service in production; the local interface keeps the
// usecase decoupled from the auth package's concrete struct so tests can
// substitute a stub.
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
// Require call. See docs/backend/library-gotchas/constructor-panics-for-non-empty-config.md.
func NewAdminGate(checker AdminChecker) *AdminGate {
	if checker == nil {
		panic("usecase: admin gate: checker must not be nil")
	}
	return &AdminGate{checker: checker}
}

// Require returns the caller's user ID after confirming the bearer is an
// admin. Sentinels (ucerr.ErrUnauthenticated, *ucerr.ForbiddenError) and
// context errors pass through unchanged; infrastructure errors are wrapped
// with callerPrefix so the log chain carries a layer-specific attribution
// (e.g. "usecase: admin role: check admin"). The wrap is applied here so
// callers no longer need a paired classifier helper.
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

// adminGateCacheKey reserves a context key for a future per-request
// memoize of the IsAdmin result. Not implemented in this PR — see
// issue #215 "Memoization seam" for the rationale.
type adminGateCacheKey struct{}
