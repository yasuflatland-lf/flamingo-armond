package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func cardgroupBatchFunc(repo repository.CardgroupRepository) dataloader.BatchFunc[string, *domain.Cardgroup] {
	return newMapKeyedBatch(func(ctx context.Context, keys []string) (map[string]*domain.Cardgroup, error) {
		return repo.FindByIDs(ctx, keys)
	}, "cardgroup")
}
