package loader

import (
	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func userBatchFunc(repo repository.UserRepository) dataloader.BatchFunc[string, *domain.User] {
	return newMapKeyedBatch(repo.FindByIDs, "user")
}
