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

// countingRepo is a function-table test double for repository.UserRepository.
// Unconfigured methods panic so an unexpected call fails loudly.
type countingRepo struct {
	findByID  func(ctx context.Context, id string) (*domain.User, error)
	findByIDs func(ctx context.Context, ids []string) (map[string]*domain.User, error)
	update    func(ctx context.Context, id string, patch repository.UserUpdate) (*domain.User, error)
}

type countingRoleRepo struct {
	findByName func(ctx context.Context, name string) (*domain.Role, error)
	findByIDs  func(ctx context.Context, ids []string) (map[string]*domain.Role, error)
}

func (r *countingRoleRepo) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	if r.findByName == nil {
		panic("countingRoleRepo.FindByName not configured")
	}
	return r.findByName(ctx, name)
}

func (r *countingRoleRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error) {
	if r.findByIDs == nil {
		panic("countingRoleRepo.FindByIDs not configured")
	}
	return r.findByIDs(ctx, ids)
}

func emptyRoleRepo() *countingRoleRepo {
	return &countingRoleRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.Role, error) {
			return map[string]*domain.Role{}, nil
		},
	}
}

func (r *countingRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if r.findByID == nil {
		panic("countingRepo.FindByID not configured")
	}
	return r.findByID(ctx, id)
}

func (r *countingRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.User, error) {
	if r.findByIDs == nil {
		panic("countingRepo.FindByIDs not configured")
	}
	return r.findByIDs(ctx, ids)
}

func (r *countingRepo) Update(ctx context.Context, id string, patch repository.UserUpdate) (*domain.User, error) {
	if r.update == nil {
		panic("countingRepo.Update not configured")
	}
	return r.update(ctx, id, patch)
}

// loadAll concurrently loads all ids through l and returns aligned results/errors.
func loadAll(ctx context.Context, l *loader.Loaders, ids []string) ([]*domain.User, []error) {
	results := make([]*domain.User, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = l.User.Load(ctx, id)()
		}(i, id)
	}
	wg.Wait()
	return results, errs
}

func TestUserLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	repo := &countingRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.User, error) {
			batchCalls.Add(1)
			out := make(map[string]*domain.User, len(ids))
			for _, id := range ids {
				out[id] = &domain.User{ID: id}
			}
			return out, nil
		},
	}

	ids := []string{"a", "b", "c", "d", "e"}
	results, errs := loadAll(context.Background(), loader.New(repo, emptyRoleRepo()), ids)

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

func TestUserLoader_PartialNotFound(t *testing.T) {
	t.Parallel()

	repo := &countingRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.User, error) {
			out := map[string]*domain.User{}
			for _, id := range ids {
				if id == "missing" {
					continue
				}
				out[id] = &domain.User{ID: id}
			}
			return out, nil
		},
	}

	ids := []string{"present-1", "missing", "present-2"}
	results, errs := loadAll(context.Background(), loader.New(repo, emptyRoleRepo()), ids)

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

func TestUserLoader_BatchFuncError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	repo := &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return nil, wantErr
		},
	}

	ids := []string{"x", "y", "z"}
	_, errs := loadAll(context.Background(), loader.New(repo, emptyRoleRepo()), ids)

	for i, err := range errs {
		if !errors.Is(err, wantErr) {
			t.Fatalf("load %d: want %v, got %v", i, wantErr, err)
		}
	}
}

func TestRoleLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	roleRepo := &countingRoleRepo{
		findByIDs: func(_ context.Context, ids []string) (map[string]*domain.Role, error) {
			batchCalls.Add(1)
			out := make(map[string]*domain.Role, len(ids))
			for _, id := range ids {
				out[id] = &domain.Role{ID: id, Name: "role-" + id}
			}
			return out, nil
		},
	}
	userRepo := &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	l := loader.New(userRepo, roleRepo)
	ids := []string{"r1", "r2", "r3"}
	var wg sync.WaitGroup
	for _, id := range ids {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			role, err := l.Role.Load(context.Background(), id)()
			if err != nil {
				t.Errorf("load role %s: %v", id, err)
				return
			}
			if role.ID != id {
				t.Errorf("role ID = %q, want %q", role.ID, id)
			}
		}()
	}
	wg.Wait()

	if got := batchCalls.Load(); got != 1 {
		t.Fatalf("Role BatchFunc should run exactly once, ran %d times", got)
	}
}

func TestMiddleware_For_Roundtrip(t *testing.T) {
	t.Parallel()

	repo := &countingRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
			return map[string]*domain.User{}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	c := e.NewContext(req, httptest.NewRecorder())

	var got *loader.Loaders
	handler := func(c *echo.Context) error {
		got = loader.For(c.Request().Context())
		return nil
	}
	if err := loader.Middleware(repo, emptyRoleRepo())(handler)(c); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if got == nil {
		t.Fatalf("loader.For returned nil; middleware did not install Loaders")
	}
	if got.User == nil {
		t.Fatalf("Loaders.User is nil")
	}
	if got.Role == nil {
		t.Fatalf("Loaders.Role is nil")
	}
}

func TestFor_NoLoaders_ReturnsNil(t *testing.T) {
	t.Parallel()
	if l := loader.For(context.Background()); l != nil {
		t.Fatalf("expected nil from bare context, got %+v", l)
	}
}
