package loader_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/labstack/echo/v5"

	"backend/internal/domain"
	"backend/internal/loader"
	"backend/internal/repository"
)

// countingRepo is a function-table test double for repository.ProfileRepository
// that records how many times each method was called. Unused methods panic so a
// test that triggers an unexpected call fails loudly instead of silently.
type countingRepo struct {
	findByID  func(ctx context.Context, id string) (*domain.Profile, error)
	findByIDs func(ctx context.Context, ids []string) (map[string]*domain.Profile, error)
	update    func(ctx context.Context, id string, patch repository.ProfileUpdate) (*domain.Profile, error)
}

func (r *countingRepo) FindByID(ctx context.Context, id string) (*domain.Profile, error) {
	if r.findByID == nil {
		panic("countingRepo.FindByID not configured")
	}
	return r.findByID(ctx, id)
}

func (r *countingRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Profile, error) {
	if r.findByIDs == nil {
		panic("countingRepo.FindByIDs not configured")
	}
	return r.findByIDs(ctx, ids)
}

func (r *countingRepo) Update(ctx context.Context, id string, patch repository.ProfileUpdate) (*domain.Profile, error) {
	if r.update == nil {
		panic("countingRepo.Update not configured")
	}
	return r.update(ctx, id, patch)
}

func TestProfileLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	repo := &countingRepo{
		findByIDs: func(ctx context.Context, ids []string) (map[string]*domain.Profile, error) {
			batchCalls.Add(1)
			out := make(map[string]*domain.Profile, len(ids))
			for _, id := range ids {
				out[id] = &domain.Profile{ID: id}
			}
			return out, nil
		},
	}
	loaders := loader.New(repo)
	ctx := context.Background()

	ids := []string{"a", "b", "c", "d", "e"}
	results := make([]*domain.Profile, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			thunk := loaders.Profile.Load(ctx, id)
			p, err := thunk()
			results[i] = p
			errs[i] = err
		}(i, id)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("load %d: unexpected error: %v", i, err)
		}
		if results[i] == nil {
			t.Fatalf("load %d: nil result", i)
		}
		if results[i].ID != ids[i] {
			t.Fatalf("load %d: ID mismatch: got %q want %q", i, results[i].ID, ids[i])
		}
	}
	if got := batchCalls.Load(); got != 1 {
		t.Fatalf("BatchFunc should run exactly once, ran %d times", got)
	}
}

func TestProfileLoader_PartialNotFound(t *testing.T) {
	t.Parallel()

	repo := &countingRepo{
		findByIDs: func(ctx context.Context, ids []string) (map[string]*domain.Profile, error) {
			out := map[string]*domain.Profile{}
			for _, id := range ids {
				if id == "missing" {
					continue
				}
				out[id] = &domain.Profile{ID: id}
			}
			return out, nil
		},
	}
	loaders := loader.New(repo)
	ctx := context.Background()

	ids := []string{"present-1", "missing", "present-2"}
	results := make([]*domain.Profile, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			p, err := loaders.Profile.Load(ctx, id)()
			results[i] = p
			errs[i] = err
		}(i, id)
	}
	wg.Wait()

	if errs[0] != nil {
		t.Fatalf("present-1: unexpected error: %v", errs[0])
	}
	if results[0] == nil || results[0].ID != "present-1" {
		t.Fatalf("present-1: bad result: %+v", results[0])
	}
	if errs[2] != nil {
		t.Fatalf("present-2: unexpected error: %v", errs[2])
	}
	if results[2] == nil || results[2].ID != "present-2" {
		t.Fatalf("present-2: bad result: %+v", results[2])
	}
	if !errors.Is(errs[1], repository.ErrNotFound) {
		t.Fatalf("missing: want ErrNotFound, got %v", errs[1])
	}
	if results[1] != nil {
		t.Fatalf("missing: want nil result, got %+v", results[1])
	}
}

func TestProfileLoader_BatchFuncError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	repo := &countingRepo{
		findByIDs: func(ctx context.Context, ids []string) (map[string]*domain.Profile, error) {
			return nil, wantErr
		},
	}
	loaders := loader.New(repo)
	ctx := context.Background()

	ids := []string{"x", "y", "z"}
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, err := loaders.Profile.Load(ctx, id)()
			errs[i] = err
		}(i, id)
	}
	wg.Wait()

	for i, err := range errs {
		if !errors.Is(err, wantErr) {
			t.Fatalf("load %d: want %v, got %v", i, wantErr, err)
		}
	}
}

func TestMiddleware_For_Roundtrip(t *testing.T) {
	t.Parallel()

	repo := &countingRepo{
		findByIDs: func(ctx context.Context, ids []string) (map[string]*domain.Profile, error) {
			return map[string]*domain.Profile{}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var got *loader.Loaders
	handler := func(c *echo.Context) error {
		got = loader.For(c.Request().Context())
		return nil
	}
	mw := loader.Middleware(repo)
	if err := mw(handler)(c); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if got == nil {
		t.Fatalf("loader.For returned nil; middleware did not install Loaders")
	}
	if got.Profile == nil {
		t.Fatalf("Loaders.Profile is nil")
	}
}

func TestFor_NoLoaders_ReturnsNil(t *testing.T) {
	t.Parallel()
	if l := loader.For(context.Background()); l != nil {
		t.Fatalf("expected nil from bare context, got %+v", l)
	}
}
