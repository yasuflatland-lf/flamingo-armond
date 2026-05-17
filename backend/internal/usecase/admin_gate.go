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

// requireAdmin returns the caller's user ID after confirming the bearer is an
// admin. Returns ucerr.ErrUnauthenticated when no caller is on the context and
// ucerr.NewForbiddenError when the caller is not an admin. Pass-through for
// context cancellation and IsAdmin infrastructure errors — no wrapping is
// applied here. Callers wrap the returned error via wrapAdminGateError.
func requireAdmin(ctx context.Context, svc AdminChecker) (callerID string, err error) {
	caller := auth.UserFrom(ctx)
	if caller == nil || caller.Sub == "" {
		return "", ucerr.ErrUnauthenticated
	}
	if svc == nil {
		return "", eris.New("usecase: admin gate: admin checker not configured")
	}
	isAdmin, err := svc.IsAdmin(ctx, caller.Sub)
	if err != nil {
		return "", err // pass-through for context errors and infrastructure failures
	}
	if !isAdmin {
		return "", ucerr.NewForbiddenError("admin only")
	}
	return caller.Sub, nil
}

// wrapAdminGateError classifies the error returned by requireAdmin. Sentinels
// (ErrUnauthenticated, ForbiddenError) and context errors pass through
// unchanged; unknown infrastructure errors are wrapped with callerPrefix so
// the log chain carries a layer-specific attribution (e.g.
// "usecase: admin role: check admin").
func wrapAdminGateError(err error, callerPrefix string) error {
	if err == nil {
		return nil
	}
	if isContextDone(err) || errors.Is(err, ucerr.ErrUnauthenticated) {
		return err
	}
	if _, ok := errors.AsType[*ucerr.ForbiddenError](err); ok {
		return err
	}
	return eris.Wrap(err, callerPrefix)
}
