package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

func cardgroupBatchFunc(repo repository.CardgroupRepository) dataloader.BatchFunc[string, *domain.Cardgroup] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Cardgroup] {
		out := make([]*dataloader.Result[*domain.Cardgroup], len(keys))

		byID, err := repo.FindByIDs(ctx, keys)
		if err != nil {
			for i := range keys {
				out[i] = &dataloader.Result[*domain.Cardgroup]{Error: err}
			}
			return out
		}

		for i, k := range keys {
			if cg, ok := byID[k]; ok {
				out[i] = &dataloader.Result[*domain.Cardgroup]{Data: cg}
				continue
			}
			out[i] = &dataloader.Result[*domain.Cardgroup]{
				Error: eris.Wrapf(repository.ErrNotFound, "cardgroup %s", k),
			}
		}
		return out
	}
}
