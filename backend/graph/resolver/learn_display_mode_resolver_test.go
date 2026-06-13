package resolver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/generated"
	"backend/graph/model"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/loader"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

type mockUpdateLearnDisplayModeUsecase struct {
	user     *domain.User
	err      error
	calls    int
	lastMode domain.LearnDisplayMode
	onSet    func(mode domain.LearnDisplayMode)
}

func (m *mockUpdateLearnDisplayModeUsecase) Set(_ context.Context, mode domain.LearnDisplayMode) (*domain.User, error) {
	m.calls++
	m.lastMode = mode
	if m.onSet != nil {
		m.onSet(mode)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func newLearnDisplayModeSrv(
	userMock *mockUserRepository,
	updateUC usecase.UpdateLearnDisplayModeUsecase,
) *handler.Server {
	userUC := usecase.NewUserUsecase(userMock, nil, nil, nil, newDiscardLogger())
	r := resolver.NewResolver(userUC, nil, nil, nil, nil, nil, nil, nil, nil, updateUC, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

func TestUserLearnDisplayMode_NoPreferenceDefaultsToFlip(t *testing.T) {
	t.Parallel()

	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: dnPtr("Alice")},
	}
	srv := newLearnDisplayModeSrv(userMock, nil)
	ctx := ctxWithBothLoaders(authedCtx("u-1"), map[string]*domain.UserPreference{}, map[string]*domain.Cardgroup{})
	body := `{"query":"{ me { id learnDisplayMode } }"}`

	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; response: %v", resp)
	}
	if me["learnDisplayMode"] != string(model.LearnDisplayModeFlipToReveal) {
		t.Fatalf("learnDisplayMode = %v, want %s", me["learnDisplayMode"], model.LearnDisplayModeFlipToReveal)
	}
}

func TestUpdateLearnDisplayMode_ReturnsUpdatedMode(t *testing.T) {
	t.Parallel()

	prefs := map[string]*domain.UserPreference{}
	mock := &mockUpdateLearnDisplayModeUsecase{
		user: &domain.User{ID: "u-1"},
		onSet: func(mode domain.LearnDisplayMode) {
			prefs["u-1"] = &domain.UserPreference{
				UserID:           "u-1",
				LearnDisplayMode: mode,
			}
		},
	}
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: dnPtr("Alice")},
	}
	srv := newLearnDisplayModeSrv(userMock, mock)
	ctx := ctxWithBothLoaders(authedCtx("u-1"), prefs, map[string]*domain.Cardgroup{})
	body := `{"query":"mutation { updateLearnDisplayMode(mode: ALWAYS_VISIBLE) { id learnDisplayMode } }"}`

	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if mock.calls != 1 {
		t.Fatalf("expected 1 Set call, got %d", mock.calls)
	}
	if mock.lastMode != domain.LearnDisplayAlwaysVisible {
		t.Fatalf("Set called with mode %q, want %q", mock.lastMode, domain.LearnDisplayAlwaysVisible)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateLearnDisplayMode"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected updateLearnDisplayMode payload, got nil; response: %v", resp)
	}
	if payload["id"] != "u-1" {
		t.Fatalf("id = %v, want u-1", payload["id"])
	}
	if payload["learnDisplayMode"] != string(model.LearnDisplayModeAlwaysVisible) {
		t.Fatalf("learnDisplayMode = %v, want %s", payload["learnDisplayMode"], model.LearnDisplayModeAlwaysVisible)
	}

	meResp := gqlRequest(t, srv, ctx, `{"query":"{ me { id learnDisplayMode } }"}`)
	if errs, hasErrs := meResp["errors"]; hasErrs {
		t.Fatalf("unexpected me errors after update: %v", errs)
	}
	meData, _ := meResp["data"].(map[string]any)
	me, _ := meData["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me after update, got nil; response: %v", meResp)
	}
	if me["learnDisplayMode"] != string(model.LearnDisplayModeAlwaysVisible) {
		t.Fatalf("me.learnDisplayMode = %v, want %s", me["learnDisplayMode"], model.LearnDisplayModeAlwaysVisible)
	}
}

func TestUpdateLearnDisplayMode_Unauthenticated(t *testing.T) {
	t.Parallel()

	mock := &mockUpdateLearnDisplayModeUsecase{err: ucerr.ErrUnauthenticated}
	userMock := &mockUserRepository{}
	srv := newLearnDisplayModeSrv(userMock, mock)
	body := `{"query":"mutation { updateLearnDisplayMode(mode: ALWAYS_VISIBLE) { id } }"}`

	resp := gqlRequest(t, srv, context.Background(), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected UNAUTHENTICATED, got %q; response: %v", code, resp)
	}
}

// TestUserLearnDisplayMode_LoadersNilReturnsInternal verifies that when the
// DataLoader middleware is absent (loader.For returns nil), the learnDisplayMode
// field resolver returns INTERNAL rather than panicking.
func TestUserLearnDisplayMode_LoadersNilReturnsInternal(t *testing.T) {
	t.Parallel()

	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: dnPtr("Alice")},
	}
	srv := newLearnDisplayModeSrv(userMock, nil)

	// No loader installed in context — loader.For(ctx) returns nil.
	body := `{"query":"{ me { id learnDisplayMode } }"}`
	resp := gqlRequest(t, srv, authedCtx("u-1"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL when loaders not installed, got %q; response: %v", code, resp)
	}
}

// TestUserLearnDisplayMode_GenericLoaderErrorReturnsInternal verifies that a
// non-sentinel UserPreference loader error maps to INTERNAL.
func TestUserLearnDisplayMode_GenericLoaderErrorReturnsInternal(t *testing.T) {
	t.Parallel()

	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: dnPtr("Alice")},
	}
	srv := newLearnDisplayModeSrv(userMock, nil)

	errorLoaders := &loader.Loaders{
		UserPreference: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[*domain.UserPreference] {
				out := make([]*dataloader.Result[*domain.UserPreference], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.UserPreference]{
						Error: errors.New("prefs store unavailable"),
					}
				}
				return out
			},
		),
		Cardgroup: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[*domain.Cardgroup] {
				out := make([]*dataloader.Result[*domain.Cardgroup], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[*domain.Cardgroup]{}
				}
				return out
			},
		),
	}
	ctx := loader.WithContext(authedCtx("u-1"), errorLoaders)

	body := `{"query":"{ me { id learnDisplayMode } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL for generic loader error, got %q; response: %v", code, resp)
	}
}

// TestUserLearnDisplayMode_ContextCancelledReturnsCancelled verifies that
// context.Canceled from the UserPreference loader maps to CANCELLED, not INTERNAL.
func TestUserLearnDisplayMode_ContextCancelledReturnsCancelled(t *testing.T) {
	t.Parallel()

	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: dnPtr("Alice")},
	}
	srv := newLearnDisplayModeSrv(userMock, nil)

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
					out[i] = &dataloader.Result[*domain.Cardgroup]{}
				}
				return out
			},
		),
	}
	ctx := loader.WithContext(authedCtx("u-1"), cancelledLoaders)

	body := `{"query":"{ me { id learnDisplayMode } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeCancelled) {
		t.Fatalf("expected CANCELLED for context.Canceled loader error, got %q; response: %v", code, resp)
	}
}
