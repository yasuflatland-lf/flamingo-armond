package usecase

import (
	"backend/internal/auth"
	"backend/internal/usecase/ucerr"
)

// requireCallerSub returns ucerr.ErrUnauthenticated bare when caller is nil or
// has an empty Sub claim. Callers MUST NOT wrap the returned error before
// returning it: gqlerr.FromUsecaseError uses errors.Is to detect the sentinel,
// and any eris.Wrap around it causes the resolver to emit INTERNAL instead of
// UNAUTHENTICATED.
func requireCallerSub(caller *auth.AuthUser) error {
	if caller == nil || caller.Sub == "" {
		return ucerr.ErrUnauthenticated
	}
	return nil
}
