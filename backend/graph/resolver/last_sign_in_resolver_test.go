package resolver_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/model"
	"backend/graph/resolver"
	"backend/internal/gqlerr"
	"backend/internal/loader"
)

// ctxWithLastSignInLoader installs an in-memory LastSignInByUserID loader.
// A user id absent from m resolves to nil data (never signed in / unknown).
func ctxWithLastSignInLoader(base context.Context, m map[string]*time.Time) context.Context {
	loaders := &loader.Loaders{
		LastSignInByUserID: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[*time.Time] {
				out := make([]*dataloader.Result[*time.Time], len(keys))
				for i, k := range keys {
					out[i] = &dataloader.Result[*time.Time]{Data: m[k]}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

func TestUserResolver_LastSignInAt_ReturnsLoaderValue(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	ctx := ctxWithLastSignInLoader(context.Background(), map[string]*time.Time{"u-1": &ts})

	got, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-1"})
	if err != nil {
		t.Fatalf("LastSignInAt: %v", err)
	}
	if got == nil || !got.Equal(ts) {
		t.Errorf("got %v, want %v", got, ts)
	}
}

func TestUserResolver_LastSignInAt_NilWhenNeverSignedIn(t *testing.T) {
	t.Parallel()

	ctx := ctxWithLastSignInLoader(context.Background(), map[string]*time.Time{})

	got, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-2"})
	if err != nil {
		t.Fatalf("LastSignInAt: %v", err)
	}
	if got != nil {
		t.Errorf("got %v, want nil (never signed in)", got)
	}
}

func TestUserResolver_LastSignInAt_MissingLoaderMiddlewareInternal(t *testing.T) {
	t.Parallel()

	_, err := (&resolver.Resolver{}).User().LastSignInAt(context.Background(), &model.User{ID: "u-1"})
	if err == nil {
		t.Fatal("want error when loader middleware is not installed, got nil")
	}
	if !gqlerr.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL wire code, got %v", err)
	}
}

// ctxWithLastSignInLoaderError installs a LastSignInByUserID loader whose batch
// function fails every key with loadErr.
func ctxWithLastSignInLoaderError(base context.Context, loadErr error) context.Context {
	loaders := &loader.Loaders{
		LastSignInByUserID: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[*time.Time] {
				out := make([]*dataloader.Result[*time.Time], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*time.Time]{Error: loadErr}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

func TestUserResolver_LastSignInAt_ContextCancelledReturnsCancelled(t *testing.T) {
	t.Parallel()

	ctx := ctxWithLastSignInLoaderError(context.Background(), context.Canceled)
	_, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-1"})
	if !gqlerr.IsCode(err, gqlerr.CodeCancelled) {
		t.Fatalf("want CANCELLED wire code for context.Canceled loader error, got %v", err)
	}
}

func TestUserResolver_LastSignInAt_GenericLoaderErrorReturnsInternal(t *testing.T) {
	t.Parallel()

	ctx := ctxWithLastSignInLoaderError(context.Background(), errors.New("db down"))
	_, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-1"})
	if !gqlerr.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL wire code for a generic loader error, got %v", err)
	}
}
