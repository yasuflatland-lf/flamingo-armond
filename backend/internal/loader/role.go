package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/rotisserie/eris"

	"backend/internal/domain"
	"backend/internal/repository"
)

func roleBatchFunc(repo repository.RoleRepository) dataloader.BatchFunc[string, *domain.Role] {
	return func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Role] {
		out := make([]*dataloader.Result[*domain.Role], len(keys))

		byID, err := repo.FindByIDs(ctx, keys)
		if err != nil {
			for i := range keys {
				out[i] = &dataloader.Result[*domain.Role]{Error: err}
			}
			return out
		}

		for i, k := range keys {
			if role, ok := byID[k]; ok {
				out[i] = &dataloader.Result[*domain.Role]{Data: role}
				continue
			}
			out[i] = &dataloader.Result[*domain.Role]{
				Error: eris.Wrapf(repository.ErrNotFound, "role %s", k),
			}
		}
		return out
	}
}
