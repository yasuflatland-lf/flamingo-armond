package loader

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"

	"backend/internal/domain"
)

type cardReader interface {
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error)
}

func cardBatchFunc(repo cardReader) dataloader.BatchFunc[string, *domain.Card] {
	return newMapKeyedBatch(repo.FindByIDs, "card")
}
