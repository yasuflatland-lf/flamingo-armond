package resolver

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/loader"
)

// Keep this file as a small landing zone for resolver-local helpers that do
// not belong to mapper.go, connection.go, or validation.go.

// loadersOrInternal returns the per-request DataLoader registry, or an
// INTERNAL wire error when the loader middleware is not installed.
func loadersOrInternal(ctx context.Context) (*loader.Loaders, *gqlerror.Error) {
	loaders := loader.For(ctx)
	if loaders == nil {
		return nil, gqlerr.Internal(ctx, eris.New("loader: middleware not installed for /query"))
	}
	return loaders, nil
}

// requireSelfOrAdmin returns nil when the caller is the target user or holds
// the admin role; otherwise it returns the appropriate GraphQL error. It owns
// the caller check, self short-circuit, role load, RoleSet build, and admin
// membership check; label attributes loader failures to the caller.
func requireSelfOrAdmin(ctx context.Context, loaders *loader.Loaders, targetID string, label string) *gqlerror.Error {
	caller := auth.UserFrom(ctx)
	if caller == nil || caller.Sub == "" {
		return gqlerr.Unauthenticated()
	}
	if caller.Sub == targetID {
		return nil
	}

	callerRoles, err := loaders.RoleByUserID.Load(ctx, caller.Sub)()
	if err != nil {
		return classifyLoaderErr(ctx, err, label)
	}
	// A resolver-local role loop would duplicate the domain membership rule.
	callerSet := make(domain.RoleSet, 0, len(callerRoles))
	for _, role := range callerRoles {
		if role != nil {
			callerSet = append(callerSet, *role)
		}
	}
	if !callerSet.ContainsAdmin() {
		return gqlerr.NewForbidden("admin only")
	}
	return nil
}

// classifyLoaderErr maps a DataLoader Load error to the wire form: CANCELLED for
// a cancelled/deadline-exceeded context, INTERNAL (wrapped with label) otherwise.
func classifyLoaderErr(ctx context.Context, err error, label string) *gqlerror.Error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return gqlerr.Cancelled(ctx, err)
	}
	return gqlerr.Internal(ctx, eris.Wrap(err, label))
}

// newNoVariantSetError builds the INTERNAL error returned when an outcome union has no
// variant set — a programming error: the usecase returned a struct with every
// field nil.
func newNoVariantSetError(ctx context.Context, name string) *gqlerror.Error {
	return gqlerr.Internal(ctx, eris.New("resolver: "+name+" has no variant set"))
}
