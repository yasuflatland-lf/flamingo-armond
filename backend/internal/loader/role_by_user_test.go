package loader_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"testing"

	"backend/internal/domain"
	"backend/internal/loader"
	"backend/internal/repository"
)

// roleBatchRepoStub satisfies repository.RoleRepository (which includes
// ListByUserIDs). Methods unrelated to the RoleByUserID loader panic so
// an unexpected call fails loudly.
type roleBatchRepoStub struct {
	listByUserIDs func(ctx context.Context, userIDs []string) (map[string][]*domain.Role, error)
}

func (s *roleBatchRepoStub) FindByName(_ context.Context, _ string) (*domain.Role, error) {
	panic("roleBatchRepoStub.FindByName not configured")
}

func (s *roleBatchRepoStub) FindByIDs(_ context.Context, _ []string) (map[string]*domain.Role, error) {
	// Return empty so the singular Role loader stays a no-op when invoked.
	return map[string]*domain.Role{}, nil
}

// AssignToUser, RevokeFromUser, ListByUser satisfy the wider RoleRepository
// interface. None of the RoleByUserID loader tests exercise these paths, so
// they panic to surface accidental coupling.
func (s *roleBatchRepoStub) AssignToUser(_ context.Context, _, _ string) error {
	panic("roleBatchRepoStub.AssignToUser not configured")
}

func (s *roleBatchRepoStub) RevokeFromUser(_ context.Context, _, _ string) error {
	panic("roleBatchRepoStub.RevokeFromUser not configured")
}

func (s *roleBatchRepoStub) ListByUser(_ context.Context, _ string) ([]*domain.Role, error) {
	panic("roleBatchRepoStub.ListByUser not configured")
}

func (s *roleBatchRepoStub) ListByUserIDs(ctx context.Context, userIDs []string) (map[string][]*domain.Role, error) {
	if s.listByUserIDs == nil {
		panic("roleBatchRepoStub.ListByUserIDs not configured")
	}
	return s.listByUserIDs(ctx, userIDs)
}

func (s *roleBatchRepoStub) ListAll(_ context.Context) ([]*domain.Role, error) {
	panic("roleBatchRepoStub.ListAll not configured")
}

// Compile-time assertion that the stub satisfies the unified interface.
var _ repository.RoleRepository = (*roleBatchRepoStub)(nil)

func newLoadersForRoleByUser(roleRepo repository.RoleRepository) *loader.Loaders {
	return loader.New(
		&countingRepo{
			findByIDs: func(_ context.Context, _ []string) (map[string]*domain.User, error) {
				return map[string]*domain.User{}, nil
			},
		},
		roleRepo,
		emptyCardgroupRepo(),
		emptyCardRepo(),
	)
}

func TestRoleByUserIDLoader_SingleUserMultipleRolesNameAsc(t *testing.T) {
	t.Parallel()

	roleA := &domain.Role{ID: "role-a", Name: "admin"}
	roleB := &domain.Role{ID: "role-b", Name: "editor"}

	repo := &roleBatchRepoStub{
		listByUserIDs: func(_ context.Context, userIDs []string) (map[string][]*domain.Role, error) {
			if len(userIDs) != 1 || userIDs[0] != "user-1" {
				t.Fatalf("unexpected userIDs: %v", userIDs)
			}
			// Caller relies on the repository to sort by name ASC.
			roles := []*domain.Role{roleA, roleB}
			sort.Slice(roles, func(i, j int) bool { return roles[i].Name < roles[j].Name })
			return map[string][]*domain.Role{"user-1": roles}, nil
		},
	}

	l := newLoadersForRoleByUser(repo)
	if l.RoleByUserID == nil {
		t.Fatal("RoleByUserID loader was not wired")
	}

	got, err := l.RoleByUserID.Load(context.Background(), "user-1")()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(roles) = %d, want 2", len(got))
	}
	if got[0].Name != "admin" || got[1].Name != "editor" {
		t.Fatalf("name order = [%q,%q], want [admin,editor]", got[0].Name, got[1].Name)
	}
}

func TestRoleByUserIDLoader_BatchesNCallsIntoOne(t *testing.T) {
	t.Parallel()

	var batchCalls atomic.Int32
	var seenKeys [][]string
	var mu sync.Mutex

	repo := &roleBatchRepoStub{
		listByUserIDs: func(_ context.Context, userIDs []string) (map[string][]*domain.Role, error) {
			batchCalls.Add(1)
			mu.Lock()
			cp := append([]string(nil), userIDs...)
			seenKeys = append(seenKeys, cp)
			mu.Unlock()
			out := make(map[string][]*domain.Role, len(userIDs))
			for _, id := range userIDs {
				out[id] = []*domain.Role{{ID: "role-for-" + id, Name: "n-" + id}}
			}
			return out, nil
		},
	}

	l := newLoadersForRoleByUser(repo)
	if l.RoleByUserID == nil {
		t.Fatal("RoleByUserID loader was not wired")
	}

	ids := []string{"u1", "u2", "u3"}
	results := make([][]*domain.Role, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = l.RoleByUserID.Load(context.Background(), id)()
		}(i, id)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("load %d: %v", i, err)
		}
		if len(results[i]) != 1 || results[i][0].ID != "role-for-"+ids[i] {
			t.Fatalf("load %d: unexpected roles %+v", i, results[i])
		}
	}
	if got := batchCalls.Load(); got != 1 {
		t.Fatalf("BatchFunc should run exactly once, ran %d times (keys seen: %v)", got, seenKeys)
	}
}

func TestRoleByUserIDLoader_UserWithNoRolesReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	repo := &roleBatchRepoStub{
		listByUserIDs: func(_ context.Context, _ []string) (map[string][]*domain.Role, error) {
			// Repository returns no entry for users with zero roles.
			return map[string][]*domain.Role{}, nil
		},
	}

	l := newLoadersForRoleByUser(repo)
	got, err := l.RoleByUserID.Load(context.Background(), "user-without-roles")()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("got nil slice; want empty (non-nil) slice so resolvers can range without nil checks")
	}
	if len(got) != 0 {
		t.Fatalf("len(roles) = %d, want 0", len(got))
	}
}

func TestRoleByUserIDLoader_MixOfKnownAndUnknownUsers(t *testing.T) {
	t.Parallel()

	repo := &roleBatchRepoStub{
		listByUserIDs: func(_ context.Context, userIDs []string) (map[string][]*domain.Role, error) {
			out := map[string][]*domain.Role{}
			for _, id := range userIDs {
				if id == "ghost" {
					continue
				}
				out[id] = []*domain.Role{{ID: "r-" + id, Name: "name-" + id}}
			}
			return out, nil
		},
	}

	l := newLoadersForRoleByUser(repo)

	ids := []string{"present", "ghost"}
	results := make([][]*domain.Role, len(ids))
	errs := make([]error, len(ids))

	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			results[i], errs[i] = l.RoleByUserID.Load(context.Background(), id)()
		}(i, id)
	}
	wg.Wait()

	if errs[0] != nil {
		t.Fatalf("present: unexpected error: %v", errs[0])
	}
	if len(results[0]) != 1 || results[0][0].ID != "r-present" {
		t.Fatalf("present: unexpected roles %+v", results[0])
	}
	if errs[1] != nil {
		t.Fatalf("ghost: want nil error (unknown user is not an error at the loader layer), got %v", errs[1])
	}
	if results[1] == nil {
		t.Fatal("ghost: want empty slice, got nil")
	}
	if len(results[1]) != 0 {
		t.Fatalf("ghost: want empty slice, got %+v", results[1])
	}
}

func TestRoleByUserIDLoader_BatchFuncErrorWraps(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("db blew up")
	repo := &roleBatchRepoStub{
		listByUserIDs: func(_ context.Context, _ []string) (map[string][]*domain.Role, error) {
			return nil, wantErr
		},
	}

	l := newLoadersForRoleByUser(repo)
	_, err := l.RoleByUserID.Load(context.Background(), "u1")()
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("want errors.Is(err, wantErr) == true; got chain: %v", err)
	}
}

// Defensive coverage for the empty-keys early return inside the batch fn.
// The dataloader runtime never invokes the batch fn with an empty slice in
// practice, so this exercises the guard directly via the package-level
// helper that the loader uses internally. We approximate by asserting that
// loading zero keys (i.e. issuing no Load calls) does not call the
// repository — i.e. the loader stays cold. This is partly tautological but
// pairs with the GORM empty-IN gotcha note in role_by_user.go.
func TestRoleByUserIDLoader_NoLoadsLeavesRepositoryCold(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	repo := &roleBatchRepoStub{
		listByUserIDs: func(_ context.Context, userIDs []string) (map[string][]*domain.Role, error) {
			calls.Add(1)
			if len(userIDs) == 0 {
				t.Fatal("ListByUserIDs called with empty slice; the GORM empty-IN gotcha would scan the whole join table")
			}
			return map[string][]*domain.Role{}, nil
		},
	}

	l := newLoadersForRoleByUser(repo)
	if l.RoleByUserID == nil {
		t.Fatal("RoleByUserID loader was not wired")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("repository called %d times before any Load; want 0", got)
	}
}
