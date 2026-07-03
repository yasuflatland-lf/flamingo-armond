package resolver

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"
	"github.com/vektah/gqlparser/v2/gqlerror"

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

// classifyLoaderErr maps a DataLoader Load error to the wire form: CANCELLED for
// a cancelled/deadline-exceeded context, INTERNAL (wrapped with label) otherwise.
func classifyLoaderErr(ctx context.Context, err error, label string) *gqlerror.Error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return gqlerr.Cancelled(ctx, err)
	}
	return gqlerr.Internal(ctx, eris.Wrap(err, label))
}

// rolesContainAdmin reports whether a batched role slice (as returned by the
// RoleByUserID DataLoader) includes the built-in admin role. Nil entries are
// skipped defensively.
func rolesContainAdmin(roles []*domain.Role) bool {
	for _, role := range roles {
		if role != nil && role.Name == domain.AdminRoleName {
			return true
		}
	}
	return false
}

// noVariantSet builds the INTERNAL error returned when an outcome union has no
// variant set — a programming error: the usecase returned a struct with every
// field nil.
func noVariantSet(ctx context.Context, name string) *gqlerror.Error {
	return gqlerr.Internal(ctx, eris.New("resolver: "+name+" has no variant set"))
}
