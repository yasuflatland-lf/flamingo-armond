package resolver_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/model"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/gqlerr/gqlerrtest"
	"backend/internal/loader"
	"backend/internal/usecase"
)

// lastSignInBatchCounter records how many batch calls the LastSignInByUserID
// loader issued and the keys carried by the most recent one, so the
// one-read-per-page property can be asserted.
type lastSignInBatchCounter struct {
	mu        sync.Mutex
	calls     int
	lastKeys  []string
	byUserID  map[string]*time.Time
	loadError error
}

func (c *lastSignInBatchCounter) batch(_ context.Context, keys []string) []*dataloader.Result[*time.Time] {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls++
	c.lastKeys = append([]string(nil), keys...)
	out := make([]*dataloader.Result[*time.Time], len(keys))
	for i, k := range keys {
		if c.loadError != nil {
			out[i] = &dataloader.Result[*time.Time]{Error: c.loadError}
			continue
		}
		out[i] = &dataloader.Result[*time.Time]{Data: c.byUserID[k]}
	}
	return out
}

// ctxWithLastSignIn installs an in-memory LastSignInByUserID loader alongside a
// RoleByUserID loader backed by rolesByUser (the field resolver's admin gate
// reads the caller's roles from the same request batch). A user id absent from
// m resolves to nil data (never signed in / unknown).
func ctxWithLastSignIn(base context.Context, m map[string]*time.Time, rolesByUser map[string][]*domain.Role) context.Context {
	return ctxWithLastSignInCounter(base, &lastSignInBatchCounter{byUserID: m}, &mockRoleByUserIDRepo{byUserID: rolesByUser})
}

// ctxWithLastSignInCounter is like ctxWithLastSignIn but accepts the
// pre-constructed backing stubs so callers can inspect their call counts after
// the request resolves.
func ctxWithLastSignInCounter(base context.Context, counter *lastSignInBatchCounter, roleRepo *mockRoleByUserIDRepo) context.Context {
	loaders := &loader.Loaders{
		LastSignInByUserID: dataloader.NewBatchedLoader(counter.batch),
		RoleByUserID: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[[]*domain.Role] {
				out := make([]*dataloader.Result[[]*domain.Role], len(keys))
				result, err := roleRepo.ListByUserIDs(ctx, keys)
				if err != nil {
					for i := range keys {
						out[i] = &dataloader.Result[[]*domain.Role]{Error: err}
					}
					return out
				}
				for i, k := range keys {
					roles := result[k]
					if roles == nil {
						roles = []*domain.Role{}
					}
					out[i] = &dataloader.Result[[]*domain.Role]{Data: roles}
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
	ctx := ctxWithLastSignIn(authedCtx("u-1"), map[string]*time.Time{"u-1": &ts}, nil)

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

	ctx := ctxWithLastSignIn(authedCtx("u-2"), map[string]*time.Time{}, nil)

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

	_, err := (&resolver.Resolver{}).User().LastSignInAt(authedCtx("u-1"), &model.User{ID: "u-1"})
	if err == nil {
		t.Fatal("want error when loader middleware is not installed, got nil")
	}
	if !gqlerrtest.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL wire code, got %v", err)
	}
}

// ctxWithLastSignInLoaderError installs a LastSignInByUserID loader whose batch
// function fails every key with loadErr.
func ctxWithLastSignInLoaderError(base context.Context, loadErr error) context.Context {
	return ctxWithLastSignInCounter(base, &lastSignInBatchCounter{loadError: loadErr}, &mockRoleByUserIDRepo{})
}

func TestUserResolver_LastSignInAt_ContextCancelledReturnsCancelled(t *testing.T) {
	t.Parallel()

	ctx := ctxWithLastSignInLoaderError(authedCtx("u-1"), context.Canceled)
	_, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-1"})
	if !gqlerrtest.IsCode(err, gqlerr.CodeCancelled) {
		t.Fatalf("want CANCELLED wire code for context.Canceled loader error, got %v", err)
	}
}

func TestUserResolver_LastSignInAt_GenericLoaderErrorReturnsInternal(t *testing.T) {
	t.Parallel()

	ctx := ctxWithLastSignInLoaderError(authedCtx("u-1"), errors.New("db down"))
	_, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-1"})
	if !gqlerrtest.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL wire code for a generic loader error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// User.lastSignInAt self-or-admin gate
// ---------------------------------------------------------------------------

// TestUserResolver_LastSignInAt_Unauthenticated verifies that an anonymous
// caller receives UNAUTHENTICATED even when the loaders are installed —
// last_sign_in_at reveals whether and when an account was used, so it is never
// readable without a caller identity.
func TestUserResolver_LastSignInAt_Unauthenticated(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	ctx := ctxWithLastSignIn(context.Background(), map[string]*time.Time{"u-1": &ts}, nil)

	_, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-1"})
	if !gqlerrtest.IsCode(err, gqlerr.CodeUnauthenticated) {
		t.Fatalf("want UNAUTHENTICATED, got %v", err)
	}
}

// TestUserResolver_LastSignInAt_NonAdminNonSelf_Forbidden verifies that a
// non-admin caller reading another user's lastSignInAt receives FORBIDDEN with
// the same "admin only" message User.roles uses, and that the timestamp does
// not leak in the returned value.
func TestUserResolver_LastSignInAt_NonAdminNonSelf_Forbidden(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	ctx := ctxWithLastSignIn(
		authedCtx("u-caller"),
		map[string]*time.Time{"u-target": &ts},
		map[string][]*domain.Role{"u-caller": {{ID: "r-general", Name: domain.GeneralRoleName}}},
	)

	got, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-target"})
	if !gqlerrtest.IsCode(err, gqlerr.CodeForbidden) {
		t.Fatalf("want FORBIDDEN, got %v", err)
	}
	if got != nil {
		t.Fatalf("want no value on FORBIDDEN, got %v", got)
	}

	// The rejection must be indistinguishable from the User.roles one: same
	// wire code, same message. Comparing against the error User.roles actually
	// produces under an equivalent context pins both halves at once.
	rolesCtx := ctxWithLastSignIn(
		authedCtx("u-caller"),
		nil,
		map[string][]*domain.Role{"u-caller": {{ID: "r-general", Name: domain.GeneralRoleName}}},
	)
	_, rolesErr := (&resolver.Resolver{}).User().Roles(rolesCtx, &model.User{ID: "u-target"})
	if err.Error() != rolesErr.Error() {
		t.Fatalf("want the User.roles rejection %q, got %q", rolesErr.Error(), err.Error())
	}
}

// TestUserResolver_LastSignInAt_Self_Allowed verifies that a non-admin caller
// reads their own lastSignInAt without a roles lookup at all.
func TestUserResolver_LastSignInAt_Self_Allowed(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	roleRepo := &mockRoleByUserIDRepo{}
	ctx := ctxWithLastSignInCounter(
		authedCtx("u-self"),
		&lastSignInBatchCounter{byUserID: map[string]*time.Time{"u-self": &ts}},
		roleRepo,
	)

	got, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-self"})
	if err != nil {
		t.Fatalf("LastSignInAt: %v", err)
	}
	if got == nil || !got.Equal(ts) {
		t.Fatalf("got %v, want %v", got, ts)
	}
	if roleRepo.listCallCount != 0 {
		t.Fatalf("self path must skip the admin check, got %d roles queries", roleRepo.listCallCount)
	}
}

// TestUserResolver_LastSignInAt_SelfNeverSignedIn_Allowed pins the nil
// (never-signed-in) case on the self path: nil is a value, not a rejection.
func TestUserResolver_LastSignInAt_SelfNeverSignedIn_Allowed(t *testing.T) {
	t.Parallel()

	ctx := ctxWithLastSignIn(authedCtx("u-self"), map[string]*time.Time{}, nil)

	got, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-self"})
	if err != nil {
		t.Fatalf("LastSignInAt: %v", err)
	}
	if got != nil {
		t.Fatalf("got %v, want nil (never signed in)", got)
	}
}

// TestUserResolver_LastSignInAt_Admin_Allowed verifies that an admin caller
// reads a foreign user's lastSignInAt, including the nil never-signed-in case.
func TestUserResolver_LastSignInAt_Admin_Allowed(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	rolesByUser := map[string][]*domain.Role{
		"admin": {{ID: "r-admin", Name: domain.AdminRoleName}},
	}

	ctx := ctxWithLastSignIn(authedCtx("admin"), map[string]*time.Time{"u-target": &ts}, rolesByUser)
	got, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-target"})
	if err != nil {
		t.Fatalf("LastSignInAt: %v", err)
	}
	if got == nil || !got.Equal(ts) {
		t.Fatalf("got %v, want %v", got, ts)
	}

	// Never-signed-in foreign user: the admin still passes the gate and reads
	// nil rather than an error.
	nilCtx := ctxWithLastSignIn(authedCtx("admin"), map[string]*time.Time{}, rolesByUser)
	gotNil, err := (&resolver.Resolver{}).User().LastSignInAt(nilCtx, &model.User{ID: "u-never"})
	if err != nil {
		t.Fatalf("LastSignInAt (never signed in): %v", err)
	}
	if gotNil != nil {
		t.Fatalf("got %v, want nil (never signed in)", gotNil)
	}
}

// TestUserResolver_LastSignInAt_AdminCheckLoaderError verifies that a failure
// of the caller's admin-status load classifies through classifyLoaderErr rather
// than falling through to the value.
func TestUserResolver_LastSignInAt_AdminCheckLoaderError(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	loaders := &loader.Loaders{
		LastSignInByUserID: dataloader.NewBatchedLoader(
			(&lastSignInBatchCounter{byUserID: map[string]*time.Time{"u-target": &ts}}).batch,
		),
		RoleByUserID: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[[]*domain.Role] {
				out := make([]*dataloader.Result[[]*domain.Role], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[[]*domain.Role]{Error: errors.New("db down")}
				}
				return out
			},
		),
	}
	ctx := loader.WithContext(authedCtx("u-caller"), loaders)

	_, err := (&resolver.Resolver{}).User().LastSignInAt(ctx, &model.User{ID: "u-target"})
	if !gqlerrtest.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL for a failed admin-status load, got %v", err)
	}
}

// usersWithLastSignInQuery requests the users list and resolves lastSignInAt
// for each row, so the batched auth.users read can be counted.
const usersWithLastSignInQuery = `{"query":"{ users(first: 3) { edges { node { id lastSignInAt } } } }"}`

// TestAdminUserResolver_LastSignInAt_OneAuthUsersReadPerPage verifies that the
// self-or-admin gate did not break batching: resolving lastSignInAt across a
// page of users still issues exactly one auth.users read, with every row's key
// in that single batch.
func TestAdminUserResolver_LastSignInAt_OneAuthUsersReadPerPage(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	mock := &mockAdminUserUsecase{
		listResult: &usecase.AdminUserConnection{
			Users: []*domain.User{
				{ID: "u1"},
				{ID: "u2"},
				{ID: "u3"},
			},
			TotalCount: 3,
		},
	}
	counter := &lastSignInBatchCounter{byUserID: map[string]*time.Time{"u1": &ts, "u3": &ts}}
	roleRepo := &mockRoleByUserIDRepo{byUserID: map[string][]*domain.Role{
		"admin": {{ID: "r-admin", Name: domain.AdminRoleName}},
	}}
	srv := newAdminUserSrv(mock)
	ctx := ctxWithLastSignInCounter(authedCtx("admin"), counter, roleRepo)

	resp := gqlRequest(t, srv, ctx, usersWithLastSignInQuery)
	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if counter.calls != 1 {
		t.Fatalf("expected exactly 1 batched auth.users read per page, got %d (keys: %v)", counter.calls, counter.lastKeys)
	}
	if len(counter.lastKeys) != 3 {
		t.Fatalf("expected the single batch to carry all 3 rows, got keys: %v", counter.lastKeys)
	}
}
