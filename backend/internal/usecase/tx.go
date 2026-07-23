package usecase

import (
	"context"

	"backend/internal/repository"
)

// txRunner runs fn inside a database transaction. The composition root wires
// the production implementation; tests may use a fake with a sentinel
// repository.Tx. Callers whose runner may be nil go through runInTx;
// constructors that panic on nil call it directly.
type txRunner func(ctx context.Context, fn func(tx repository.Tx) error) error

// runInTx executes fn inside run's transaction. A nil run invokes fn with a nil
// handle for tests that inject repository fakes; production always wires a real
// runner. Use this from methods whose transaction is a correctness requirement,
// so test constructors without a database do not need to fabricate one.
func runInTx(ctx context.Context, run txRunner, fn func(tx repository.Tx) error) error {
	if run == nil {
		return fn(nil)
	}
	return run(ctx, fn)
}

// newTxRunner returns the production txRunner backed by db, or nil when db is
// nil so callers can leave the field unset for explicit-tx test constructors.
func newTxRunner(db repository.Tx) txRunner {
	if db == nil {
		return nil
	}
	return func(ctx context.Context, fn func(tx repository.Tx) error) error {
		return db.WithContext(ctx).Transaction(fn)
	}
}
