package loader

import (
	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func cardgroupBatchFunc(repo repository.CardgroupRepository) dataloader.BatchFunc[string, *domain.Cardgroup] {
	return newMapKeyedBatch(repo.FindByIDs, "cardgroup")
}
