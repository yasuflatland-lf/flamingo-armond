package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func roleBatchFunc(repo repository.RoleRepository) dataloader.BatchFunc[string, *domain.Role] {
	return newMapKeyedBatch(func(ctx context.Context, keys []string) (map[string]*domain.Role, error) {
		return repo.FindByIDs(ctx, keys)
	}, "role")
}
