package usecase

import (
	"context"

	"gorm.io/gorm"
)

// txRunner is the function the usecase calls to run fn inside a database
// transaction. The composition root (cmd/server) wires the real
// *gorm.DB.Transaction-backed implementation; unit tests pass a fake that
// invokes fn with a sentinel *gorm.DB. Used by CardUsecase, SwipeUsecase,
// cardImportUsecase, and MasterNotionSyncUsecase.
type txRunner func(ctx context.Context, fn func(tx *gorm.DB) error) error

// newTxRunner returns the production txRunner backed by db, or nil when db is
// nil so callers can leave the field unset for explicit-tx test constructors.
func newTxRunner(db *gorm.DB) txRunner {
	if db == nil {
		return nil
	}
	return func(ctx context.Context, fn func(tx *gorm.DB) error) error {
		return db.WithContext(ctx).Transaction(fn)
	}
}
