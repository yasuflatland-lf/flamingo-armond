package usecase

import (
	"context"

	"gorm.io/gorm"
)

// txRunner is the function the usecase calls to run fn inside a database
// transaction. The composition root (cmd/server) wires the real
// *gorm.DB.Transaction-backed implementation; unit tests pass a fake that
// invokes fn with a sentinel *gorm.DB. Used by CardUsecase, SwipeUsecase,
// dictionaryUsecase, and NotionSyncUsecase.
type txRunner func(ctx context.Context, fn func(tx *gorm.DB) error) error
