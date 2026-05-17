package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

func swipeRecordBatchFunc(repo repository.SwipeRecordRepository) dataloader.BatchFunc[string, *domain.SwipeRecord] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*domain.SwipeRecord] {
		out := make([]*dataloader.Result[*domain.SwipeRecord], len(keys))

		// Defensive: dataloader normally never invokes the batch fn with an
		// empty key slice, but the GORM "WHERE id IN ()" gotcha would turn
		// such a call into a full-table scan. Short-circuit instead.
		if len(keys) == 0 {
			return out
		}

		byID, err := repo.FindByIDs(ctx, keys)
		if err != nil {
			for i := range keys {
				out[i] = &dataloader.Result[*domain.SwipeRecord]{Error: err}
			}
			return out
		}

		for i, k := range keys {
			if sr, ok := byID[k]; ok {
				out[i] = &dataloader.Result[*domain.SwipeRecord]{Data: sr}
				continue
			}
			out[i] = &dataloader.Result[*domain.SwipeRecord]{
				Error: eris.Wrapf(repository.ErrNotFound, "swipe record %s", k),
			}
		}
		return out
	}
}
