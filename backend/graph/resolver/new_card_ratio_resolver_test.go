package resolver_test

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

type mockUpdateNewCardRatioUsecase struct {
	user    *domain.User
	err     error
	calls   int
	lastNum int
	lastDen int
	onSet   func(num, den int)
}

func (m *mockUpdateNewCardRatioUsecase) Set(_ context.Context, numerator, denominator int) (*domain.User, error) {
	m.calls++
	m.lastNum = numerator
	m.lastDen = denominator
	if m.onSet != nil {
		m.onSet(numerator, denominator)
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}

func newNewCardRatioSrv(
	userMock *mockUserRepository,
	updateUC usecase.UpdateNewCardRatioUsecase,
) *handler.Server {
	userUC := usecase.NewUserUsecase(userMock, nil, nil, newDiscardLogger())
	r := resolver.NewResolver(userUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, updateUC, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// mustNewCardRatio builds a validated ratio for fixtures; a bad literal is a
// test bug, so fail hard rather than thread the error through.
func mustNewCardRatio(t *testing.T, num, den int) domain.NewCardRatio {
	t.Helper()
	r, err := domain.ParseNewCardRatio(num, den)
	if err != nil {
		t.Fatalf("ParseNewCardRatio(%d, %d): %v", num, den, err)
	}
	return r
}

func TestUserNewCardRatio_NoPreferenceDefaultsToFourFifths(t *testing.T) {
	t.Parallel()

	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: dnPtr("Alice")},
	}
	srv := newNewCardRatioSrv(userMock, nil)
	ctx := ctxWithBothLoaders(authedCtx("u-1"), map[string]*domain.UserPreference{}, map[string]*domain.Cardgroup{})
	body := `{"query":"{ me { id newCardRatio { numerator denominator } } }"}`

	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	ratio := meNewCardRatio(t, resp)
	if ratio["numerator"] != float64(domain.DefaultNewCardRatio.Numerator()) ||
		ratio["denominator"] != float64(domain.DefaultNewCardRatio.Denominator()) {
		t.Fatalf("newCardRatio = %v, want default %d/%d", ratio,
			domain.DefaultNewCardRatio.Numerator(), domain.DefaultNewCardRatio.Denominator())
	}
}

func TestUserNewCardRatio_StoredPreferenceReturnsStoredFraction(t *testing.T) {
	t.Parallel()

	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: dnPtr("Alice")},
	}
	srv := newNewCardRatioSrv(userMock, nil)
	prefs := map[string]*domain.UserPreference{
		"u-1": {UserID: "u-1", NewCardRatio: mustNewCardRatio(t, 3, 7)},
	}
	ctx := ctxWithBothLoaders(authedCtx("u-1"), prefs, map[string]*domain.Cardgroup{})
	body := `{"query":"{ me { id newCardRatio { numerator denominator } } }"}`

	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	ratio := meNewCardRatio(t, resp)
	if ratio["numerator"] != float64(3) || ratio["denominator"] != float64(7) {
		t.Fatalf("newCardRatio = %v, want stored 3/7", ratio)
	}
}

func TestUpdateNewCardRatio_ReturnsUpdatedRatio(t *testing.T) {
	t.Parallel()

	prefs := map[string]*domain.UserPreference{}
	mock := &mockUpdateNewCardRatioUsecase{
		user: &domain.User{ID: "u-1"},
		onSet: func(num, den int) {
			// 3/7 is already reduced, so ParseNewCardRatio round-trips it.
			ratio := mustNewCardRatio(t, num, den)
			prefs["u-1"] = &domain.UserPreference{UserID: "u-1", NewCardRatio: ratio}
		},
	}
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-1", DisplayName: dnPtr("Alice")},
	}
	srv := newNewCardRatioSrv(userMock, mock)
	ctx := ctxWithBothLoaders(authedCtx("u-1"), prefs, map[string]*domain.Cardgroup{})
	body := `{"query":"mutation { updateNewCardRatio(numerator: 3, denominator: 7) { id newCardRatio { numerator denominator } } }"}`

	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if mock.calls != 1 {
		t.Fatalf("expected 1 Set call, got %d", mock.calls)
	}
	if mock.lastNum != 3 || mock.lastDen != 7 {
		t.Fatalf("Set called with %d/%d, want 3/7", mock.lastNum, mock.lastDen)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateNewCardRatio"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected updateNewCardRatio payload, got nil; response: %v", resp)
	}
	if payload["id"] != "u-1" {
		t.Fatalf("id = %v, want u-1", payload["id"])
	}
	ratio, _ := payload["newCardRatio"].(map[string]any)
	if ratio == nil || ratio["numerator"] != float64(3) || ratio["denominator"] != float64(7) {
		t.Fatalf("newCardRatio = %v, want 3/7", ratio)
	}
}

// TestUpdateNewCardRatio_UsecaseValidationErrorMapsToBadUserInput verifies the
// resolver forwards a usecase ValidationError to the wire as BAD_USER_INPUT,
// propagating the usecase-attributed extensions.field. The resolver no longer
// parses the ratio or derives the field itself — that moved into the usecase,
// which is exercised directly in update_new_card_ratio_test.go.
func TestUpdateNewCardRatio_UsecaseValidationErrorMapsToBadUserInput(t *testing.T) {
	t.Parallel()

	mock := &mockUpdateNewCardRatioUsecase{
		err: ucerr.NewValidationError("numerator", "invalid new-card ratio"),
	}
	userMock := &mockUserRepository{}
	srv := newNewCardRatioSrv(userMock, mock)
	body := `{"query":"mutation { updateNewCardRatio(numerator: 5, denominator: 5) { id } }"}`

	resp := gqlRequest(t, srv, authedCtx("u-1"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected BAD_USER_INPUT, got %q; response: %v", code, resp)
	}
	if field, _ := errExtensions(t, resp)["field"].(string); field != "numerator" {
		t.Fatalf("expected extensions.field = numerator, got %q; response: %v", field, resp)
	}
}

// meNewCardRatio extracts data.me.newCardRatio from a GraphQL response, failing
// the test if the path is absent.
func meNewCardRatio(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; response: %v", resp)
	}
	ratio, _ := me["newCardRatio"].(map[string]any)
	if ratio == nil {
		t.Fatalf("expected data.me.newCardRatio, got nil; response: %v", resp)
	}
	return ratio
}
