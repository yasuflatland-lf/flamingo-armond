package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

func userBatchFunc(repo repository.UserRepository) dataloader.BatchFunc[string, *domain.User] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*domain.User] {
		out := make([]*dataloader.Result[*domain.User], len(keys))

		// Defensive: dataloader normally never invokes the batch fn with an
		// empty key slice, but the GORM "WHERE id IN ()" gotcha would turn
		// such a call into a full-table scan. Short-circuit instead.
		if len(keys) == 0 {
			return out
		}

		byID, err := repo.FindByIDs(ctx, keys)
		if err != nil {
			for i := range keys {
				out[i] = &dataloader.Result[*domain.User]{Error: err}
			}
			return out
		}

		for i, k := range keys {
			if user, ok := byID[k]; ok {
				out[i] = &dataloader.Result[*domain.User]{Data: user}
				continue
			}
			out[i] = &dataloader.Result[*domain.User]{
				Error: eris.Wrapf(repository.ErrNotFound, "user %s", k),
			}
		}
		return out
	}
}
