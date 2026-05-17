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
// context cancellation; wraps any other AdminChecker error.
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
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", eris.Wrap(err, "usecase: admin gate: check admin")
	}
	if !isAdmin {
		return "", ucerr.NewForbiddenError("admin only")
	}
	return caller.Sub, nil
}
