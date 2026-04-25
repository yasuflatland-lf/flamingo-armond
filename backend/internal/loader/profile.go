package loader

import (
	"context"
	"fmt"

	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func profileBatchFunc(repo repository.ProfileRepository) dataloader.BatchFunc[string, *domain.Profile] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Profile] {
		out := make([]*dataloader.Result[*domain.Profile], len(keys))

		byID, err := repo.FindByIDs(ctx, keys)
		if err != nil {
			for i := range keys {
				out[i] = &dataloader.Result[*domain.Profile]{Error: err}
			}
			return out
		}

		for i, k := range keys {
			p, ok := byID[k]
			if !ok {
				out[i] = &dataloader.Result[*domain.Profile]{
					Error: fmt.Errorf("profile %s: %w", k, repository.ErrNotFound),
				}
				continue
			}
			out[i] = &dataloader.Result[*domain.Profile]{Data: p}
		}
		return out
	}
}
