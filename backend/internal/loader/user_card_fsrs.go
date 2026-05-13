package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func userCardFSRSBatchFunc(repo repository.UserCardFSRSRepository, viewer string) dataloader.BatchFunc[string, *domain.UserCardFSRS] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*domain.UserCardFSRS] {
		out := make([]*dataloader.Result[*domain.UserCardFSRS], len(keys))
		if len(keys) == 0 {
			return out
		}

		byCardID, err := repo.FindByUserAndCardIDs(ctx, viewer, keys)
		if err != nil {
			for i := range keys {
				out[i] = &dataloader.Result[*domain.UserCardFSRS]{Error: err}
			}
			return out
		}

		for i, k := range keys {
			out[i] = &dataloader.Result[*domain.UserCardFSRS]{Data: byCardID[k]}
		}
		return out
	}
}
