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
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, uc, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ctxWithCardgroupLoader returns a context enriched with a loader.Loaders
// whose Cardgroup loader is backed by an in-memory map. Used for tests that
// exercise error branches on the Cardgroup loader but do not need a
// UserPreference loader (the UserPreference field is left nil, meaning these
// tests must also provide a UserPreference loader via ctxWithBothLoaders).
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

// ctxWithBothLoaders returns a context enriched with a loader.Loaders whose
// UserPreference loader is backed by an in-memory map keyed by user_id and
// whose Cardgroup loader is backed by an in-memory map keyed by cardgroup_id.
// Use this helper whenever the test exercises the full two-step chain:
// UserPreferenceLoader → CardgroupLoader.
//
// A missing user_id in prefs returns nil data with nil error (matching
// production loader behaviour — absence is a normal state, not an error).
// A missing cardgroup_id in cgs returns ErrNotFound — matching the Cardgroup
// loader's production behaviour, which differs from UserPreference intentionally.
func ctxWithBothLoaders(
	base context.Context,
	prefs map[string]*domain.UserPreference,
	cgs map[string]*domain.Cardgroup,
) context.Context {
	loaders := &loader.Loaders{
		UserPreference: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[*domain.UserPreference] {
				out := make([]*dataloader.Result[*domain.UserPreference], len(keys))
				for i, k := range keys {
					if pref, ok := prefs[k]; ok {
						out[i] = &dataloader.Result[*domain.UserPreference]{Data: pref}
						continue
					}
					// Missing key: nil data + nil error, matching production loader.
					out[i] = &dataloader.Result[*domain.UserPreference]{}
				}
				return out
			},
		),
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

	mock := &mockLastViewedCardgroupUsecase{
		setResult: &domain.User{ID: "u-1"},
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

// newMeServer builds a gqlgen handler.Server wired to the given UserUsecase
// for testing User field resolvers via the me query.
func newMeServer(userMock *mockUserRepository) *handler.Server {
	uc := usecase.NewUserUsecase(userMock)
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// TestUserLastViewedCardgroup_NoPreferenceRowReturnsNull verifies case: no
// preference row → null. When the UserPreference loader returns nil data + nil
// error (no preference row for this user), the field resolver returns (nil, nil)
// — GraphQL null with no error.
func TestUserLastViewedCardgroup_NoPreferenceRowReturnsNull(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

	// UserPreference loader has no entry for "u-1" → ErrNotFound path.
	ctx := ctxWithBothLoaders(
		authedCtx("u-1"),
		map[string]*domain.UserPreference{}, // no row for u-1
		map[string]*domain.Cardgroup{},
	)
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

// TestUserLastViewedCardgroup_NilCardgroupIDReturnsNull verifies case: preference
// row exists but LastViewedCardgroupID is nil → null. When the UserPreference
// loader returns a preference row whose LastViewedCardgroupID is nil, the field
// resolver returns (nil, nil).
func TestUserLastViewedCardgroup_NilCardgroupIDReturnsNull(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

	// Preference row exists but LastViewedCardgroupID is nil.
	prefs := map[string]*domain.UserPreference{
		"u-1": {UserID: "u-1", LastViewedCardgroupID: nil},
	}
	ctx := ctxWithBothLoaders(authedCtx("u-1"), prefs, map[string]*domain.Cardgroup{})
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
		t.Fatalf("expected lastViewedCardgroup == null when pref.LastViewedCardgroupID is nil, got %v",
			me["lastViewedCardgroup"])
	}
}

// TestUserLastViewedCardgroup_PopulatedResolvesViaDataLoader verifies case:
// preference row with a non-nil cardgroup ID and the cardgroup present → hydrated
// model. When both a preference row and the cardgroup itself are present, the
// field resolver returns the correctly hydrated Cardgroup model.
func TestUserLastViewedCardgroup_PopulatedResolvesViaDataLoader(t *testing.T) {
	t.Parallel()

	cgID := "cg-1"
	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

	prefs := map[string]*domain.UserPreference{
		"u-1": {UserID: "u-1", LastViewedCardgroupID: &cgID},
	}
	cgs := map[string]*domain.Cardgroup{
		cgID: {ID: cgID, OwnerID: "u-1", Name: "My Group"},
	}
	ctx := ctxWithBothLoaders(authedCtx("u-1"), prefs, cgs)
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

// TestUserLastViewedCardgroup_DanglingIDResolvesNull verifies case: dangling
// cardgroup FK → null. When the UserPreference loader returns a preference with
// a non-nil LastViewedCardgroupID but CardgroupLoader.Load returns ErrNotFound
// (e.g. ON DELETE SET NULL race between preference read and field resolution),
// the resolver gracefully returns (nil, nil) — GraphQL null with no error.
func TestUserLastViewedCardgroup_DanglingIDResolvesNull(t *testing.T) {
	t.Parallel()

	cgID := "cg-deleted"
	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

	prefs := map[string]*domain.UserPreference{
		"u-1": {UserID: "u-1", LastViewedCardgroupID: &cgID},
	}
	// Empty cardgroup map: cgID will resolve to ErrNotFound.
	ctx := ctxWithBothLoaders(authedCtx("u-1"), prefs, map[string]*domain.Cardgroup{})
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

// ---------------------------------------------------------------------------
// User.lastViewedCardgroup error-branch tests
// ---------------------------------------------------------------------------

// TestUserLastViewedCardgroup_LoadersNilReturnsInternal verifies that when the
// DataLoader middleware was not installed (loader.For returns nil), the resolver
// returns a GraphQL INTERNAL error rather than panicking or returning null.
func TestUserLastViewedCardgroup_LoadersNilReturnsInternal(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

	// Deliberately omit the loader installation so loader.For returns nil.
	body := `{"query":"{ me { id lastViewedCardgroup { id } } }"}`
	resp := gqlRequest(t, srv, authedCtx("u-1"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL when loaders not installed, got %q", code)
	}
}

// TestUserLastViewedCardgroup_ContextCancelledReturnsCancelled verifies that
// when the UserPreference DataLoader's BatchFn propagates context.Canceled,
// the resolver maps it to gqlerr.Cancelled (extension code "CANCELLED") rather
// than INTERNAL.
func TestUserLastViewedCardgroup_ContextCancelledReturnsCancelled(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

	// Install a UserPreference loader whose BatchFn always returns context.Canceled.
	cancelledLoaders := &loader.Loaders{
		UserPreference: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[*domain.UserPreference] {
				out := make([]*dataloader.Result[*domain.UserPreference], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.UserPreference]{Error: context.Canceled}
				}
				return out
			},
		),
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
// an arbitrary (non-sentinel) UserPreference DataLoader error maps to
// gqlerr.Internal.
func TestUserLastViewedCardgroup_GenericLoaderErrorReturnsInternal(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

	// Install a UserPreference loader whose BatchFn always returns a generic error.
	errorLoaders := &loader.Loaders{
		UserPreference: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[*domain.UserPreference] {
				out := make([]*dataloader.Result[*domain.UserPreference], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.UserPreference]{
						Error: errBoom,
					}
				}
				return out
			},
		),
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
