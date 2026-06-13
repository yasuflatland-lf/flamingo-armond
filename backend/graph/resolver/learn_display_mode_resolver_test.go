package resolver_test

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"backend/graph/generated"
	"backend/graph/model"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/gqlerr"
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
	userUC := usecase.NewUserUsecase(userMock, nil, nil, newDiscardLogger())
	r := resolver.NewResolver(userUC, nil, nil, nil, nil, nil, nil, nil, nil, updateUC, nil, nil)
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
