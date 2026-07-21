package resolver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/model"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/gqlerr/gqlerrtest"
	"backend/internal/loader"
)

// ctxWithUserLoaderError installs a User loader whose batch function fails every
// key with loadErr.
func ctxWithUserLoaderError(base context.Context, loadErr error) context.Context {
	loaders := &loader.Loaders{
		User: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[*domain.User] {
				out := make([]*dataloader.Result[*domain.User], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.User]{Error: loadErr}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

// TestCardgroupResolver_Owner_MissingLoaderMiddlewareInternal asserts the
// nil-registry guard (now centralized in loadersOrInternal) still maps to
// INTERNAL.
func TestCardgroupResolver_Owner_MissingLoaderMiddlewareInternal(t *testing.T) {
	t.Parallel()

	_, err := (&resolver.Resolver{}).Cardgroup().Owner(context.Background(), &model.Cardgroup{OwnerID: "u-1"})
	if err == nil {
		t.Fatal("want error when loader middleware is not installed, got nil")
	}
	if !gqlerrtest.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL wire code, got %v", err)
	}
}

// TestCardgroupResolver_Owner_ContextCancelledReturnsCancelled pins the
// deliberate drift fix: a cancelled context mid-DataLoader now maps to
// CANCELLED (previously this site collapsed straight to INTERNAL).
func TestCardgroupResolver_Owner_ContextCancelledReturnsCancelled(t *testing.T) {
	t.Parallel()

	ctx := ctxWithUserLoaderError(context.Background(), context.Canceled)
	_, err := (&resolver.Resolver{}).Cardgroup().Owner(ctx, &model.Cardgroup{OwnerID: "u-1"})
	if !gqlerrtest.IsCode(err, gqlerr.CodeCancelled) {
		t.Fatalf("want CANCELLED wire code for context.Canceled loader error, got %v", err)
	}
}

// TestCardgroupResolver_Owner_GenericLoaderErrorReturnsInternal verifies that a
// generic loader error still maps to INTERNAL.
func TestCardgroupResolver_Owner_GenericLoaderErrorReturnsInternal(t *testing.T) {
	t.Parallel()

	ctx := ctxWithUserLoaderError(context.Background(), errors.New("db down"))
	_, err := (&resolver.Resolver{}).Cardgroup().Owner(ctx, &model.Cardgroup{OwnerID: "u-1"})
	if !gqlerrtest.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL wire code for a generic loader error, got %v", err)
	}
}
