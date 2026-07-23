package usecase

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestRunInTx_NilRunner_InvokesFunction(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("function failed")
	calls := 0
	err := runInTx(context.Background(), nil, func(tx *gorm.DB) error {
		calls++
		if tx != nil {
			t.Fatalf("tx = %p, want nil", tx)
		}
		return wantErr
	})

	if err != wantErr {
		t.Fatalf("runInTx() error = %v, want %v", err, wantErr)
	}
	if calls != 1 {
		t.Fatalf("function calls = %d, want 1", calls)
	}
}

func TestNewTxRunner_NilDB_ReturnsNil(t *testing.T) {
	t.Parallel()

	if got := newTxRunner(nil); got != nil {
		t.Fatalf("newTxRunner(nil) = non-nil txRunner, want nil")
	}
}

func TestNewTxRunner_NonNilDB_ReturnsRunner(t *testing.T) {
	t.Parallel()

	// A zero-value *gorm.DB is enough to assert the returned func is non-nil.
	// The runner is never invoked here, so no live connection is dereferenced.
	if got := newTxRunner(&gorm.DB{}); got == nil {
		t.Fatalf("newTxRunner(&gorm.DB{}) = nil txRunner, want non-nil")
	}
}
