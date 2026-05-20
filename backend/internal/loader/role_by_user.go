package loader

import (
	"context"
	"errors"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

// RoleByUserIDLoader batches role lookups by user ID. Each key maps to the
// ordered list of roles assigned to that user (empty slice when the user has
// no roles, and also when the user ID is unknown — a missing user is not an
// error at the loader layer).
type RoleByUserIDLoader = dataloader.Loader[string, []*domain.Role]

func roleByUserIDBatchFunc(repo repository.UserRoleRepository) dataloader.BatchFunc[string, []*domain.Role] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[[]*domain.Role] {
		out := make([]*dataloader.Result[[]*domain.Role], len(keys))

		// Defensive: dataloader normally never invokes the batch fn with an
		// empty key slice, but the GORM "WHERE id IN ()" gotcha would turn
		// such a call into a full-table scan. Short-circuit instead.
		if len(keys) == 0 {
			return out
		}

		byUser, err := repo.ListByUserIDs(ctx, keys)
		if err != nil {
			batchErr := err
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				batchErr = eris.Wrap(err, "loader: list roles by user IDs")
			}
			for i := range keys {
				out[i] = &dataloader.Result[[]*domain.Role]{Error: batchErr}
			}
			return out
		}

		for i, k := range keys {
			roles, ok := byUser[k]
			if !ok {
				// Unknown or role-less user: return an explicit empty slice
				// so resolvers can range over it without nil checks.
				roles = []*domain.Role{}
			}
			out[i] = &dataloader.Result[[]*domain.Role]{Data: roles}
		}
		return out
	}
}
