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

// runInTx executes fn inside run's transaction. A nil run — the shape
// newTxRunner produces when the composition root has no *gorm.DB, which happens
// only in tests that inject repository fakes — invokes fn directly with a nil
// handle: the fakes ignore it, and production always wires a real runner. Use
// this from methods whose transaction is a correctness requirement rather than
// an optional optimisation, so a test constructor without a database does not
// have to fabricate one.
func runInTx(ctx context.Context, run txRunner, fn func(tx *gorm.DB) error) error {
	if run == nil {
		return fn(nil)
	}
	return run(ctx, fn)
}

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
