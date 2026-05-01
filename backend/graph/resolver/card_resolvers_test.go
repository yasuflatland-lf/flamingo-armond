package resolver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"gorm.io/gorm"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// --- mock repositories used only by the card resolver tests ---

// cardMockRepo satisfies usecase.CardRepository. Only the methods exercised by
// the two resolver mutations are non-trivial; the rest are no-op stubs.
type cardMockRepo struct {
	deleteByIDsResult int64
	deleteByIDsErr    error
}

func (m *cardMockRepo) FindByID(_ context.Context, _ string) (*domain.Card, error) {
	return nil, nil
}
func (m *cardMockRepo) FindByIDs(_ context.Context, _ []string) (map[string]*domain.Card, error) {
	return nil, nil
}
func (m *cardMockRepo) FindByCardgroup(_ context.Context, _ string) ([]*domain.Card, error) {
	return nil, nil
}
func (m *cardMockRepo) FindPageByCardgroup(
	_ context.Context,
	_ string,
	_, _ *repository.CardCursor,
	_, _ int,
	_ repository.CardOrderBy,
	_ repository.SortOrder,
) ([]*domain.Card, int64, error) {
	return nil, 0, nil
}
func (m *cardMockRepo) Create(_ context.Context, _ *domain.Card) error { return nil }
func (m *cardMockRepo) Update(_ context.Context, _ string, _ repository.CardUpdate) (*domain.Card, error) {
	return nil, nil
}
func (m *cardMockRepo) Delete(_ context.Context, _ string) error { return nil }
func (m *cardMockRepo) DeleteByIDsTx(
	_ context.Context, _ *gorm.DB, _ string, _ []string,
) (int64, error) {
	return m.deleteByIDsResult, m.deleteByIDsErr
}

// cardMockCGRepo satisfies usecase.CardgroupRepositoryForCard.
type cardMockCGRepo struct {
	findResult *domain.Cardgroup
	findErr    error
}

func (m *cardMockCGRepo) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.findResult, m.findErr
}

// --- construction helpers ---

// cardFakeTx returns a txRunner stub that executes fn with a nil *gorm.DB.
// Mock repositories ignore the tx argument, so this suffices for tests that
// exercise the BulkDelete happy path without a real database.
func cardFakeTx() func(context.Context, func(*gorm.DB) error) error {
	return func(_ context.Context, fn func(*gorm.DB) error) error {
		return fn(nil)
	}
}

// newCardSrv builds a gqlgen handler backed by a resolver wired with the
// supplied mocks and an in-process tx runner.
func newCardSrv(
	cardRepo usecase.CardRepository,
	cgRepo usecase.CardgroupRepositoryForCard,
	tx func(context.Context, func(*gorm.DB) error) error,
) *handler.Server {
	cardUC := usecase.NewCardUsecaseWithTx(cardRepo, cgRepo, tx)
	r := resolver.NewResolver(nil, nil, cardUC, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// errExtensions extracts response.errors[0].extensions as a map.
func errExtensions(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	errs, ok := resp["errors"].([]any)
	if !ok || len(errs) == 0 {
		t.Fatalf("expected errors in response, got %v", resp)
	}
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	return ext
}

// --- test cases ---

// TestResolver_DeleteCards_HappyPath verifies that deleteCards returns the
// number of rows deleted when the caller is authenticated.
func TestResolver_DeleteCards_HappyPath(t *testing.T) {
	t.Parallel()

	srv := newCardSrv(
		&cardMockRepo{deleteByIDsResult: 2},
		&cardMockCGRepo{},
		cardFakeTx(),
	)

	body := `{"query":"mutation { deleteCards(ids: [\"c1\",\"c2\"]) }"}`
	resp := gqlRequest(t, srv, authedCtx("u1"), body)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	// json.Unmarshal decodes GraphQL Int! as float64.
	got, ok := data["deleteCards"].(float64)
	if !ok {
		t.Fatalf("expected data.deleteCards to be numeric, got %T %v",
			data["deleteCards"], data["deleteCards"])
	}
	if int(got) != 2 {
		t.Fatalf("expected data.deleteCards == 2, got %v", got)
	}
}

// TestResolver_DeleteCards_Anonymous verifies that an unauthenticated request
// returns errors[0].extensions.code == "UNAUTHENTICATED".
func TestResolver_DeleteCards_Anonymous(t *testing.T) {
	t.Parallel()

	srv := newCardSrv(&cardMockRepo{}, &cardMockCGRepo{}, cardFakeTx())

	body := `{"query":"mutation { deleteCards(ids: [\"c1\",\"c2\"]) }"}`
	resp := gqlRequest(t, srv, context.Background(), body)

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// TestResolver_CreateCard_PartialFSRS_BadUserInput verifies that passing only
// some FSRS override fields surfaces errors[0].extensions.code ==
// "BAD_USER_INPUT" with extensions.field == "input.fsrs". The cardgroup
// ownership check is satisfied, so the error originates solely from the
// partial-override validation inside the usecase.
func TestResolver_CreateCard_PartialFSRS_BadUserInput(t *testing.T) {
	t.Parallel()

	// The cardgroup is owned by the authenticated user so auth passes.
	cgRepo := &cardMockCGRepo{
		findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"},
	}
	// cardRepo.Create must NOT be reached; validation fails before persistence.
	srv := newCardSrv(&cardMockRepo{}, cgRepo, cardFakeTx())

	// Only stability is provided; the other eight FSRS fields are absent.
	// This triggers ErrFSRSOverridePartial -> BAD_USER_INPUT on "input.fsrs".
	mutation := map[string]any{
		"query": `mutation($input: NewCardInput!) {
			createCard(input: $input) { card { id } }
		}`,
		"variables": map[string]any{
			"input": map[string]any{
				"cardgroupId": "cg1",
				"front":       "Question",
				"back":        "Answer",
				"stability":   7.5,
			},
		},
	}
	bodyBytes, _ := json.Marshal(mutation)
	resp := gqlRequest(t, srv, authedCtx("u1"), string(bodyBytes))

	ext := errExtensions(t, resp)
	code, _ := ext["code"].(string)
	if code != "BAD_USER_INPUT" {
		t.Fatalf("expected BAD_USER_INPUT, got %q", code)
	}
	field, _ := ext["field"].(string)
	if field != "input.fsrs" {
		t.Fatalf("expected extensions.field == \"input.fsrs\", got %q", field)
	}
}
