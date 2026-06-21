package usecase

import (
	"testing"

	"gorm.io/gorm"
)

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
