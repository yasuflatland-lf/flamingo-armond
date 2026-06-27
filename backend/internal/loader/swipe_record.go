package loader

import (
	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func swipeRecordBatchFunc(repo repository.SwipeRecordRepository) dataloader.BatchFunc[string, *domain.SwipeRecord] {
	return newMapKeyedBatch(repo.FindByIDs, "swipe record")
}
