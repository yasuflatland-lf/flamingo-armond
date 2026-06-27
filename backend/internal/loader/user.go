package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func userBatchFunc(repo repository.UserRepository) dataloader.BatchFunc[string, *domain.User] {
	return newMapKeyedBatch(func(ctx context.Context, keys []string) (map[string]*domain.User, error) {
		return repo.FindByIDs(ctx, keys)
	}, "user")
}
