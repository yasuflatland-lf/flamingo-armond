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
	"backend/internal/usecase/ucerr"
)

// mockLastViewedCardgroupUsecase stubs LastViewedCardgroupUsecase.
type mockLastViewedCardgroupUsecase struct {
	setOutcome      usecase.SetLastViewedCardgroupOutcome
	setErr          error
	setCalls        int
	lastCardgroupID string
}

func (m *mockLastViewedCardgroupUsecase) Set(_ context.Context, cardgroupID string) (usecase.SetLastViewedCardgroupOutcome, error) {
	m.setCalls++
	m.lastCardgroupID = cardgroupID
	return m.setOutcome, m.setErr
}

// newLastViewedSrv builds a gqlgen Server wired to uc; other usecase fields are nil.
func newLastViewedSrv(uc usecase.LastViewedCardgroupUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, uc, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ctxWithBothLoaders installs in-memory UserPreference and Cardgroup loaders.
// A missing user_id returns nil data + nil error (absence is not an error).
// A missing cardgroup_id returns ErrNotFound (matching production behaviour).
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
					} else {
						out[i] = &dataloader.Result[*domain.UserPreference]{}
					}
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
					} else {
						out[i] = &dataloader.Result[*domain.Cardgroup]{Error: repository.ErrNotFound}
					}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

// setLastViewedMutation selects across both variants of the
// SetLastViewedCardgroupResult union so a single mutation body covers the
// success and input-validation cases.
const setLastViewedMutation = `{"query":"mutation { setLastViewedCardgroup(cardgroupId: \"cg-1\") { __typename ... on SetLastViewedCardgroupSuccess { user { id } } ... on InputValidationError { field message } } }"}`

func TestSetLastViewedCardgroup_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockLastViewedCardgroupUsecase{
		setOutcome: usecase.SetLastViewedCardgroupOutcome{
			User: &domain.User{ID: "u-1"},
		},
	}
	srv := newLastViewedSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("u-1"), setLastViewedMutation)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["setLastViewedCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.setLastViewedCardgroup, got nil; response: %v", resp)
	}
	if payload["__typename"] != "SetLastViewedCardgroupSuccess" {
		t.Fatalf("expected __typename=SetLastViewedCardgroupSuccess, got %v", payload["__typename"])
	}
	user, _ := payload["user"].(map[string]any)
	if user == nil {
		t.Fatalf("expected user payload, got nil; response: %v", resp)
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

// TestSetLastViewedCardgroup_InputValidation verifies that a validation failure
// (e.g. cardgroup not found or not owned) is surfaced as the
// InputValidationError union variant — returned as data, not as a GraphQL
// error. This is the "errors as data" pattern promoted in Phase 3.
func TestSetLastViewedCardgroup_InputValidation(t *testing.T) {
	t.Parallel()

	mock := &mockLastViewedCardgroupUsecase{
		setOutcome: usecase.SetLastViewedCardgroupOutcome{
			Validation: &usecase.InputValidationInfo{
				Field:   "cardgroupId",
				Message: "cardgroup not found or not owned",
			},
		},
	}
	srv := newLastViewedSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("u-1"), setLastViewedMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["setLastViewedCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.setLastViewedCardgroup, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v", payload["__typename"])
	}
	if payload["field"] != "cardgroupId" {
		t.Fatalf("expected field=cardgroupId, got %v", payload["field"])
	}
	if payload["message"] != "cardgroup not found or not owned" {
		t.Fatalf("expected message='cardgroup not found or not owned', got %v", payload["message"])
	}
}

func TestSetLastViewedCardgroup_Unauthenticated(t *testing.T) {
	t.Parallel()

	mock := &mockLastViewedCardgroupUsecase{
		setErr: ucerr.ErrUnauthenticated,
	}
	srv := newLastViewedSrv(mock)
	resp := gqlRequest(t, srv, context.Background(), setLastViewedMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// newMeServer builds a gqlgen Server wired to UserUsecase for testing User
// field resolvers via the me query.
func newMeServer(userMock *mockUserRepository) *handler.Server {
	uc := usecase.NewUserUsecase(userMock)
	r := resolver.NewResolver(uc, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// TestUserLastViewedCardgroup_NoPreferenceRowReturnsNull verifies that no
// preference row (nil data, nil error from the loader) resolves to GraphQL null.
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

// TestUserLastViewedCardgroup_NilCardgroupIDReturnsNull verifies that a
// preference row with nil LastViewedCardgroupID resolves to GraphQL null.
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

// TestUserLastViewedCardgroup_PopulatedResolvesViaDataLoader verifies that a
// non-nil LastViewedCardgroupID with the cardgroup present returns the hydrated model.
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

// TestUserLastViewedCardgroup_DanglingIDResolvesNull verifies that a non-nil
// LastViewedCardgroupID whose cardgroup no longer exists (ErrNotFound from loader)
// resolves to GraphQL null with no error.
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

// TestUserLastViewedCardgroup_LoadersNilReturnsInternal verifies that when the
// DataLoader middleware is absent (loader.For returns nil), the resolver returns
// INTERNAL rather than panicking.
func TestUserLastViewedCardgroup_LoadersNilReturnsInternal(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

	body := `{"query":"{ me { id lastViewedCardgroup { id } } }"}`
	resp := gqlRequest(t, srv, authedCtx("u-1"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL when loaders not installed, got %q", code)
	}
}

// TestUserLastViewedCardgroup_ContextCancelledReturnsCancelled verifies that
// context.Canceled from the UserPreference loader maps to CANCELLED, not INTERNAL.
func TestUserLastViewedCardgroup_ContextCancelledReturnsCancelled(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

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
// a non-sentinel UserPreference loader error maps to INTERNAL.
func TestUserLastViewedCardgroup_GenericLoaderErrorReturnsInternal(t *testing.T) {
	t.Parallel()

	dn := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: &dn},
	}
	srv := newMeServer(userMock)

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

var errBoom = errors.New("boom")
