package loader

import (
	"context"
	"errors"
	"time"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/rotisserie/eris"
)

// LastSignInByUserIDLoader batches auth.users.last_sign_in_at lookups by user
// ID. Each key maps to a *time.Time: nil when the user has never signed in
// (NULL column) or the user id is unknown — a missing row is not an error at
// the loader layer.
type LastSignInByUserIDLoader = dataloader.Loader[string, *time.Time]

// lastSignInReader is the narrow read interface the loader needs from the user
// repository.
type lastSignInReader interface {
	LastSignInByUserIDs(ctx context.Context, ids []string) (map[string]*time.Time, error)
}

func lastSignInByUserIDBatchFunc(repo lastSignInReader) dataloader.BatchFunc[string, *time.Time] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*time.Time] {
		out := make([]*dataloader.Result[*time.Time], len(keys))

		// Defensive: dataloader normally never invokes the batch fn with an
		// empty key slice, but the GORM "WHERE id IN ()" gotcha would turn
		// such a call into a full-table scan. Short-circuit instead.
		if len(keys) == 0 {
			return out
		}

		byUser, err := repo.LastSignInByUserIDs(ctx, keys)
		if err != nil {
			batchErr := err
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				batchErr = eris.Wrap(err, "loader: last sign-in by user IDs")
			}
			for i := range keys {
				out[i] = &dataloader.Result[*time.Time]{Error: batchErr}
			}
			return out
		}

		for i, k := range keys {
			// Unknown or never-signed-in user both map to nil; a missing row is
			// not an error at the loader layer.
			out[i] = &dataloader.Result[*time.Time]{Data: byUser[k]}
		}
		return out
	}
}
