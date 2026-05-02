package resolver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/loader"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// ---------------------------------------------------------------------------
// mockLastViewedCardgroupUsecase — stub for LastViewedCardgroupUsecase
// ---------------------------------------------------------------------------

type mockLastViewedCardgroupUsecase struct {
	setResult       *domain.User
	setErr          error
	setCalls        int
	lastCardgroupID string
}

func (m *mockLastViewedCardgroupUsecase) Set(_ context.Context, cardgroupID string) (*domain.User, error) {
	m.setCalls++
	m.lastCardgroupID = cardgroupID
	return m.setResult, m.setErr
}

// newLastViewedSrv builds a gqlgen handler.Server backed by a mock
// LastViewedCardgroupUsecase. Other usecase fields are nil — only the
// last-viewed-cardgroup paths are exercised here.
func newLastViewedSrv(uc usecase.LastViewedCardgroupUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, uc)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ctxWithCardgroupLoader returns a context enriched with a loader.Loaders
// whose Cardgroup loader is backed by an in-memory map. Used for tests that
// resolve User.lastViewedCardgroup (which goes through the DataLoader).
//
// The not-found case returns repository.ErrNotFound so the resolver can
// branch on it and surface null instead of INTERNAL.
func ctxWithCardgroupLoader(base context.Context, cgs map[string]*domain.Cardgroup) context.Context {
	loaders := &loader.Loaders{
		Cardgroup: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Cardgroup] {
				out := make([]*dataloader.Result[*domain.Cardgroup], len(keys))
				for i, k := range keys {
					if cg, ok := cgs[k]; ok {
						out[i] = &dataloader.Result[*domain.Cardgroup]{Data: cg}
						continue
					}
					out[i] = &dataloader.Result[*domain.Cardgroup]{Error: repository.ErrNotFound}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

// ---------------------------------------------------------------------------
// Mutation.setLastViewedCardgroup tests
// ---------------------------------------------------------------------------

const setLastViewedMutation = `{"query":"mutation { setLastViewedCardgroup(cardgroupId: \"cg-1\") { id } }"}`

// TestSetLastViewedCardgroup_HappyPath verifies that the resolver returns the
// usecase's user payload as model.User on success.
func TestSetLastViewedCardgroup_HappyPath(t *testing.T) {
	t.Parallel()

	cgID := "cg-1"
	mock := &mockLastViewedCardgroupUsecase{
		setResult: &domain.User{ID: "u-1", LastViewedCardgroupID: &cgID},
	}
	srv := newLastViewedSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("u-1"), setLastViewedMutation)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	user, _ := data["setLastViewedCardgroup"].(map[string]any)
	if user == nil {
		t.Fatalf("expected data.setLastViewedCardgroup, got nil; response: %v", resp)
	}
	if user["id"] != "u-1" {
		t.Fatalf("expected id=u-1, got %v", user["id"])
	}
	if mock.setCalls != 1 {
		t.Fatalf("expected 1 Set call, got %d", mock.setCalls)
	}
	if mock.lastCardgroupID != "cg-1" {
		t.Fatalf("Set called with cardgroupID=%q, want %q", mock.lastCardgroupID, "cg-1")
	}
}

// TestSetLastViewedCardgroup_BadUserInput verifies that BAD_USER_INPUT from
// the usecase propagates to the caller with extensions.code and field set.
func TestSetLastViewedCardgroup_BadUserInput(t *testing.T) {
	t.Parallel()

	mock := &mockLastViewedCardgroupUsecase{
		setErr: gqlerr.BadUserInput("cardgroupId", "cardgroup not found or not owned"),
	}
	srv := newLastViewedSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("u-1"), setLastViewedMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected BAD_USER_INPUT, got %q", code)
	}
	errs, _ := resp["errors"].([]any)
	first, _ := errs[0].(map[string]any)
	ext, _ := first["extensions"].(map[string]any)
	if ext["field"] != "cardgroupId" {
		t.Fatalf("expected extensions.field=cardgroupId, got %v", ext["field"])
	}
}

// TestSetLastViewedCardgroup_Unauthenticated verifies that an anonymous
// caller receives UNAUTHENTICATED from the usecase via the resolver.
func TestSetLastViewedCardgroup_Unauthenticated(t *testing.T) {
	t.Parallel()

	mock := &mockLastViewedCardgroupUsecase{
		setErr: gqlerr.Unauthenticated(),
	}
	srv := newLastViewedSrv(mock)
	resp := gqlRequest(t, srv, context.Background(), setLastViewedMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// ---------------------------------------------------------------------------
// User.lastViewedCardgroup field resolver tests
// ---------------------------------------------------------------------------

// TestUserLastViewedCardgroup_NilFieldResolvesNull verifies that a user with
// LastViewedCardgroupID == nil resolves the field as GraphQL null with no
// DataLoader call. The me { lastViewedCardgroup { id } } query is convenient
// here because Me is wired through UserUsecase.
func TestUserLastViewedCardgroup_NilFieldResolvesNull(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn}, // LastViewedCardgroupID nil
	}
	uc := usecase.NewUserUsecase(userMock)
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	// Empty cardgroup loader — the resolver must not invoke it.
	ctx := ctxWithCardgroupLoader(authedCtx("u-1"), map[string]*domain.Cardgroup{})
	body := `{"query":"{ me { id lastViewedCardgroup { id } } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; response: %v", resp)
	}
	if _, exists := me["lastViewedCardgroup"]; !exists {
		t.Fatalf("expected lastViewedCardgroup key (null), got absent; response: %v", resp)
	}
	if me["lastViewedCardgroup"] != nil {
		t.Fatalf("expected lastViewedCardgroup == null, got %v", me["lastViewedCardgroup"])
	}
}

// TestUserLastViewedCardgroup_PopulatedResolvesViaDataLoader verifies that a
// user with a non-nil LastViewedCardgroupID resolves the field by calling
// the Cardgroup DataLoader with that ID.
func TestUserLastViewedCardgroup_PopulatedResolvesViaDataLoader(t *testing.T) {
	t.Parallel()

	cgID := "cg-1"
	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{
			ID:                    "u-1",
			DisplayName:           &dn,
			LastViewedCardgroupID: &cgID,
		},
	}
	uc := usecase.NewUserUsecase(userMock)
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	cgs := map[string]*domain.Cardgroup{
		cgID: {ID: cgID, OwnerID: "u-1", Name: "My Group"},
	}
	ctx := ctxWithCardgroupLoader(authedCtx("u-1"), cgs)
	body := `{"query":"{ me { id lastViewedCardgroup { id name } } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; response: %v", resp)
	}
	cg, _ := me["lastViewedCardgroup"].(map[string]any)
	if cg == nil {
		t.Fatalf("expected lastViewedCardgroup populated, got nil; response: %v", resp)
	}
	if cg["id"] != cgID {
		t.Fatalf("expected cardgroup id=%q, got %v", cgID, cg["id"])
	}
	if cg["name"] != "My Group" {
		t.Fatalf("expected cardgroup name=%q, got %v", "My Group", cg["name"])
	}
}

// ---------------------------------------------------------------------------
// User.lastViewedCardgroup error-branch tests
// ---------------------------------------------------------------------------

// TestUserLastViewedCardgroup_LoadersNilReturnsInternal verifies that when the
// DataLoader middleware was not installed (loader.For returns nil), the resolver
// returns a GraphQL INTERNAL error rather than panicking or returning null.
func TestUserLastViewedCardgroup_LoadersNilReturnsInternal(t *testing.T) {
	t.Parallel()

	cgID := "cg-1"
	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{
			ID:                    "u-1",
			DisplayName:           &dn,
			LastViewedCardgroupID: &cgID,
		},
	}
	uc := usecase.NewUserUsecase(userMock)
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	// Deliberately omit the loader installation so loader.For returns nil.
	body := `{"query":"{ me { id lastViewedCardgroup { id } } }"}`
	resp := gqlRequest(t, srv, authedCtx("u-1"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL when loaders not installed, got %q", code)
	}
}

// TestUserLastViewedCardgroup_ContextCancelledReturnsCancelled verifies that
// when the DataLoader's BatchFn propagates context.Canceled, the resolver maps
// it to gqlerr.Cancelled (extension code "CANCELLED") rather than INTERNAL.
func TestUserLastViewedCardgroup_ContextCancelledReturnsCancelled(t *testing.T) {
	t.Parallel()

	cgID := "cg-1"
	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{
			ID:                    "u-1",
			DisplayName:           &dn,
			LastViewedCardgroupID: &cgID,
		},
	}
	uc := usecase.NewUserUsecase(userMock)
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	// Install a Cardgroup loader whose BatchFn always returns context.Canceled.
	cancelledLoaders := &loader.Loaders{
		Cardgroup: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Cardgroup] {
				out := make([]*dataloader.Result[*domain.Cardgroup], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.Cardgroup]{Error: context.Canceled}
				}
				return out
			},
		),
	}
	ctx := loader.WithContext(authedCtx("u-1"), cancelledLoaders)

	body := `{"query":"{ me { id lastViewedCardgroup { id } } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeCancelled) {
		t.Fatalf("expected CANCELLED for context.Canceled loader error, got %q", code)
	}
}

// TestUserLastViewedCardgroup_GenericLoaderErrorReturnsInternal verifies that
// an arbitrary (non-sentinel) DataLoader error maps to gqlerr.Internal.
func TestUserLastViewedCardgroup_GenericLoaderErrorReturnsInternal(t *testing.T) {
	t.Parallel()

	cgID := "cg-1"
	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{
			ID:                    "u-1",
			DisplayName:           &dn,
			LastViewedCardgroupID: &cgID,
		},
	}
	uc := usecase.NewUserUsecase(userMock)
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	// Install a Cardgroup loader whose BatchFn always returns a generic error.
	errorLoaders := &loader.Loaders{
		Cardgroup: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Cardgroup] {
				out := make([]*dataloader.Result[*domain.Cardgroup], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.Cardgroup]{
						Error: errBoom,
					}
				}
				return out
			},
		),
	}
	ctx := loader.WithContext(authedCtx("u-1"), errorLoaders)

	body := `{"query":"{ me { id lastViewedCardgroup { id } } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL for generic loader error, got %q", code)
	}
}

// errBoom is a package-level sentinel used by TestUserLastViewedCardgroup_GenericLoaderErrorReturnsInternal.
var errBoom = errors.New("boom")

// TestUserLastViewedCardgroup_DanglingIDResolvesNull verifies that when the
// DataLoader returns ErrNotFound (e.g. ON DELETE SET NULL race between
// model conversion and field resolution), the resolver gracefully renders
// null rather than surfacing an error.
func TestUserLastViewedCardgroup_DanglingIDResolvesNull(t *testing.T) {
	t.Parallel()

	cgID := "cg-deleted"
	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{
			ID:                    "u-1",
			DisplayName:           &dn,
			LastViewedCardgroupID: &cgID, // points to a cardgroup the loader cannot find
		},
	}
	uc := usecase.NewUserUsecase(userMock)
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	// Empty loader: the cgID will resolve to repository.ErrNotFound.
	ctx := ctxWithCardgroupLoader(authedCtx("u-1"), map[string]*domain.Cardgroup{})
	body := `{"query":"{ me { id lastViewedCardgroup { id } } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors for dangling FK: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; response: %v", resp)
	}
	if me["lastViewedCardgroup"] != nil {
		t.Fatalf("expected lastViewedCardgroup == null on dangling FK, got %v",
			me["lastViewedCardgroup"])
	}
}
