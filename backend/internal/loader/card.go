package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

func cardBatchFunc(repo repository.CardRepository) dataloader.BatchFunc[string, *domain.Card] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Card] {
		out := make([]*dataloader.Result[*domain.Card], len(keys))

		byID, err := repo.FindByIDs(ctx, keys)
		if err != nil {
			for i := range keys {
				out[i] = &dataloader.Result[*domain.Card]{Error: err}
			}
			return out
		}

		for i, k := range keys {
			if card, ok := byID[k]; ok {
				out[i] = &dataloader.Result[*domain.Card]{Data: card}
				continue
			}
			out[i] = &dataloader.Result[*domain.Card]{
				Error: eris.Wrapf(repository.ErrNotFound, "card %s", k),
			}
		}
		return out
	}
}
