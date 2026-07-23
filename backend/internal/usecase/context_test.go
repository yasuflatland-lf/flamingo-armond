package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestWrapInfraErr_ContextErrorsPassThrough(t *testing.T) {
	t.Parallel()

	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		err := err
		t.Run(err.Error(), func(t *testing.T) {
			t.Parallel()

			got := wrapInfraErr(err, "usecase: test: operation")

			if !errors.Is(got, err) {
				t.Fatalf("expected errors.Is(%v, %v) to be true", got, err)
			}
			if got != err {
				t.Fatalf("expected identical context error, got %T: %v", got, got)
			}
		})
	}
}

func TestWrapInfraErr_WrapsOtherErrors(t *testing.T) {
	t.Parallel()

	base := errors.New("database unavailable")
	got := wrapInfraErr(base, "usecase: test: operation")

	if !errors.Is(got, base) {
		t.Fatalf("expected wrapped error to contain base error, got %v", got)
	}
	if got == base {
		t.Fatal("expected infrastructure error to be wrapped")
	}
	assertInternalChain(t, got, "usecase: test: operation")
}
