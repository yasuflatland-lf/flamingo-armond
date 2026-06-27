package repository

// White-box tests for refetchAfterUpdate, the shared tail of the aggregate Update
// methods. The helper is unexported so the tests live in the same package. No live
// DB is required — the refetch closure is a fake.

import (
	"errors"
	"strings"
	"testing"
)

func TestRefetchAfterUpdate(t *testing.T) {
	t.Parallel()

	type row struct{ id string }

	notFound := errors.New("not found")
	refetchFail := errors.New("boom")

	t.Run("rows affected zero returns notFoundErr without refetching", func(t *testing.T) {
		t.Parallel()

		called := false
		got, err := refetchAfterUpdate(0, notFound,
			func() (*row, error) { called = true; return &row{id: "x"}, nil }, "")
		if got != nil {
			t.Fatalf("got = %v, want nil", got)
		}
		if !errors.Is(err, notFound) {
			t.Fatalf("err = %v, want notFound", err)
		}
		if called {
			t.Fatalf("refetch was called for a zero-rows update")
		}
	})

	t.Run("rows affected positive returns the refetched row", func(t *testing.T) {
		t.Parallel()

		want := &row{id: "abc"}
		got, err := refetchAfterUpdate(1, notFound,
			func() (*row, error) { return want, nil }, "")
		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if got != want {
			t.Fatalf("got = %v, want %v", got, want)
		}
	})

	t.Run("refetch error with prefix is wrapped and preserves the cause", func(t *testing.T) {
		t.Parallel()

		const prefix = "repository: role: update: find after update"
		got, err := refetchAfterUpdate(1, notFound,
			func() (*row, error) { return nil, refetchFail }, prefix)
		if got != nil {
			t.Fatalf("got = %v, want nil", got)
		}
		if !errors.Is(err, refetchFail) {
			t.Fatalf("err = %v, want chain to refetchFail", err)
		}
		if !strings.Contains(err.Error(), prefix) {
			t.Fatalf("err = %q, want it to contain prefix %q", err.Error(), prefix)
		}
	})

	t.Run("refetch error with empty prefix is returned unwrapped", func(t *testing.T) {
		t.Parallel()

		got, err := refetchAfterUpdate(1, notFound,
			func() (*row, error) { return nil, refetchFail }, "")
		if got != nil {
			t.Fatalf("got = %v, want nil", got)
		}
		if err != refetchFail {
			t.Fatalf("err = %v, want the exact refetchFail error (unwrapped)", err)
		}
	})
}
