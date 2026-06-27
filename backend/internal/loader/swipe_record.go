package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func swipeRecordBatchFunc(repo repository.SwipeRecordRepository) dataloader.BatchFunc[string, *domain.SwipeRecord] {
	return newMapKeyedBatch(func(ctx context.Context, keys []string) (map[string]*domain.SwipeRecord, error) {
		return repo.FindByIDs(ctx, keys)
	}, "swipe record")
}
