package loader

import (
	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
	"backend/internal/repository"
)

func roleBatchFunc(repo repository.RoleRepository) dataloader.BatchFunc[string, *domain.Role] {
	return newMapKeyedBatch(repo.FindByIDs, "role")
}
