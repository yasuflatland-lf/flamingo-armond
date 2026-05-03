package loader_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"gorm.io/gorm"

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

func (r *countingRoleRepo) FindByID(_ context.Context, _ string) (*domain.Role, error) {
	panic("countingRoleRepo.FindByID not configured")
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

func (r *countingRoleRepo) Create(_ context.Context, _ string) (*domain.Role, error) {
	panic("countingRoleRepo.Create not configured")
}

func (r *countingRoleRepo) Update(_ context.Context, _, _ string) (*domain.Role, error) {
	panic("countingRoleRepo.Update not configured")
}

func (r *countingRoleRepo) Delete(_ context.Context, _ string) error {
	panic("countingRoleRepo.Delete not configured")
}

// AssignToUser, RevokeFromUser, ListByUser satisfy the wider RoleRepository
// interface. The cardgroup/card/user loader tests never assign or revoke
// roles, so these panic to surface accidental coupling.
func (r *countingRoleRepo) AssignToUser(_ context.Context, _, _ string) error {
	panic("countingRoleRepo.AssignToUser not configured")
}

func (r *countingRoleRepo) RevokeFromUser(_ context.Context, _, _ string) error {
	panic("countingRoleRepo.RevokeFromUser not configured")
}

func (r *countingRoleRepo) ListByUser(_ context.Context, _ string) ([]*domain.Role, error) {
	panic("countingRoleRepo.ListByUser not configured")
}

func (r *countingRoleRepo) ListByUserIDs(_ context.Context, _ []string) (map[string][]*domain.Role, error) {
	panic("countingRoleRepo.ListByUserIDs not configured")
}

func (r *countingRoleRepo) ListAll(_ context.Context) ([]*domain.Role, error) {
	panic("countingRoleRepo.ListAll not configured")
}

func (r *countingRoleRepo) CountAdminUsers(_ context.Context) (int64, error) {
	panic("countingRoleRepo.CountAdminUsers not configured")
}

func emptyRoleRepo() *countingRoleRepo {
	return &countingRoleRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.Role, error) {
			return map[string]*domain.Role{}, nil
		},
	}
}

// countingCardgroupRepo is a minimal test double for repository.CardgroupRepository.
type countingCardgroupRepo struct {
	findByIDs func(ctx context.Context, ids []string) (map[string]*domain.Cardgroup, error)
}

type countingCardRepo struct {
	findByIDs func(ctx context.Context, ids []string) (map[string]*domain.Card, error)
}

func (r *countingCardgroupRepo) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	panic("countingCardgroupRepo.FindByID not configured")
}
func (r *countingCardgroupRepo) FindByOwner(_ context.Context, _ string) ([]*domain.Cardgroup, error) {
	panic("countingCardgroupRepo.FindByOwner not configured")
}
func (r *countingCardgroupRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Cardgroup, error) {
	if r.findByIDs == nil {
		panic("countingCardgroupRepo.FindByIDs not configured")
	}
	return r.findByIDs(ctx, ids)
}
func (r *countingCardgroupRepo) Create(_ context.Context, _ *domain.Cardgroup) error {
	panic("countingCardgroupRepo.Create not configured")
}
func (r *countingCardgroupRepo) Update(_ context.Context, _ string, _ repository.CardgroupUpdate) (*domain.Cardgroup, error) {
	panic("countingCardgroupRepo.Update not configured")
}
func (r *countingCardgroupRepo) Delete(_ context.Context, _ string) error {
	panic("countingCardgroupRepo.Delete not configured")
}

func emptyCardgroupRepo() *countingCardgroupRepo {
	return &countingCardgroupRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.Cardgroup, error) {
			return map[string]*domain.Cardgroup{}, nil
		},
	}
}

func (r *countingCardRepo) FindByID(_ context.Context, _ string) (*domain.Card, error) {
	panic("countingCardRepo.FindByID not configured")
}
func (r *countingCardRepo) FindByIDTx(_ context.Context, _ *gorm.DB, _ string) (*domain.Card, error) {
	panic("countingCardRepo.FindByIDTx not configured")
}
func (r *countingCardRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error) {
	if r.findByIDs == nil {
		panic("countingCardRepo.FindByIDs not configured")
	}
	return r.findByIDs(ctx, ids)
}
func (r *countingCardRepo) FindByCardgroup(_ context.Context, _ string) ([]*domain.Card, error) {
	panic("countingCardRepo.FindByCardgroup not configured")
}
func (r *countingCardRepo) FindPageByCardgroup(
	_ context.Context, _ string,
	_, _ *repository.CardCursor,
	_, _ int,
	_ repository.CardOrderBy, _ repository.SortOrder,
) ([]*domain.Card, int64, error) {
	panic("countingCardRepo.FindPageByCardgroup not configured")
}
func (r *countingCardRepo) FindDueCardsTx(_ context.Context, _ *gorm.DB, _ string, _ time.Time, _ int) ([]*domain.Card, error) {
	panic("countingCardRepo.FindDueCardsTx not configured")
}
func (r *countingCardRepo) Create(_ context.Context, _ *domain.Card) error {
	panic("countingCardRepo.Create not configured")
}
func (r *countingCardRepo) UpdateFSRSStateTx(_ context.Context, _ *gorm.DB, _ string, _ domain.FSRSState) error {
	panic("countingCardRepo.UpdateFSRSStateTx not configured")
}
func (r *countingCardRepo) Update(_ context.Context, _ string, _ repository.CardUpdate) (*domain.Card, error) {
	panic("countingCardRepo.Update not configured")
}
func (r *countingCardRepo) Delete(_ context.Context, _ string) error {
	panic("countingCardRepo.Delete not configured")
}
func (r *countingCardRepo) DeleteByIDsTx(_ context.Context, _ *gorm.DB, _ string, _ []string) (int64, error) {
	panic("countingCardRepo.DeleteByIDsTx not configured")
}
func (r *countingCardRepo) UpsertManyTx(_ context.Context, _ *gorm.DB, _ []*domain.Card) (repository.UpsertManyTxResult, error) {
	panic("countingCardRepo.UpsertManyTx not configured")
}
func (r *countingCardRepo) FindByCardgroupAndFront(_ context.Context, _, _ string) (*domain.Card, error) {
	panic("countingCardRepo.FindByCardgroupAndFront not configured")
}

func emptyCardRepo() *countingCardRepo {
	return &countingCardRepo{
		findByIDs: func(_ context.Context, _ []string) (map[string]*domain.Card, error) {
			return map[string]*domain.Card{}, nil
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

// ListPage satisfies repository.UserRepository. The loader-layer tests never
// hit cursor pagination, so this fixture panics if called — surfacing any
// accidental coupling instead of silently returning a fabricated empty page.
func (r *countingRepo) ListPage(
	_ context.Context,
	_, _ *string,
	_, _ int,
	_ *string,
) ([]*domain.User, int64, error) {
	panic("countingRepo.ListPage not configured")
}

// SetLastViewedCardgroup satisfies repository.UserRepository. Loader-layer
// tests never invoke this path; panic if called so accidental coupling is
// surfaced rather than silently no-oped.
func (r *countingRepo) SetLastViewedCardgroup(_ context.Context, _, _ string) error {
	panic("countingRepo.SetLastViewedCardgroup not configured")
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
	results, errs := loadAll(context.Background(), loader.New(repo, emptyRoleRepo(), emptyCardgroupRepo(), emptyCardRepo()), ids)

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
	results, errs := loadAll(context.Background(), loader.New(repo, emptyRoleRepo(), emptyCardgroupRepo(), emptyCardRepo()), ids)

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
	_, errs := loadAll(context.Background(), loader.New(repo, emptyRoleRepo(), emptyCardgroupRepo(), emptyCardRepo()), ids)

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

	l := loader.New(userRepo, roleRepo, emptyCardgroupRepo(), emptyCardRepo())
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
	if err := loader.Middleware(repo, emptyRoleRepo(), emptyCardgroupRepo(), emptyCardRepo())(handler)(c); err != nil {
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
	if got.Cardgroup == nil {
		t.Fatalf("Loaders.Cardgroup is nil")
	}
}

func TestFor_NoLoaders_ReturnsNil(t *testing.T) {
	t.Parallel()
	if l := loader.For(context.Background()); l != nil {
		t.Fatalf("expected nil from bare context, got %+v", l)
	}
}
