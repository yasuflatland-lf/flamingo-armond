package resolver_test

import (
	"context"
	"encoding/json"
	"math/rand"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"gorm.io/gorm"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/domain/service"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// --- mock repositories used only by the card resolver tests ---

// cardMockRepo satisfies usecase.CardRepository and usecase.CardRepoForLearn.
// FindByID, Update, and FindDueCardsForUser are configurable via struct fields;
// the remaining methods are no-op stubs.
type cardMockRepo struct {
	deleteByIDsResult int64
	deleteByIDsErr    error
	findDueRows       []domain.DueCard
	findDueErr        error
	findDueLimit      int

	// Fields used by UpdateCard tests.
	findByIDResult *domain.Card
	findByIDErr    error
	updateResult   *domain.Card
	updateErr      error
}

func (m *cardMockRepo) FindByID(_ context.Context, _ string) (*domain.Card, error) {
	return m.findByIDResult, m.findByIDErr
}
func (m *cardMockRepo) FindDueCardsForUser(_ context.Context, _ string, _ string, _ time.Time, _ time.Time, limit int) ([]domain.DueCard, error) {
	m.findDueLimit = limit
	return m.findDueRows, m.findDueErr
}
func (m *cardMockRepo) FindPageByCardgroupForUser(
	_ context.Context,
	_, _ string,
	_, _ *repository.CardCursor,
	_, _ int,
	_ repository.CardOrderBy,
	_ repository.SortOrder,
	_ *string,
) ([]*domain.Card, int64, error) {
	return nil, 0, nil
}
func (m *cardMockRepo) FindByCardgroupAndFront(_ context.Context, _, _ string) (*domain.Card, error) {
	return nil, nil
}
func (m *cardMockRepo) Create(_ context.Context, _ *domain.Card) error { return nil }
func (m *cardMockRepo) Update(_ context.Context, _ string, _ repository.CardUpdate) (*domain.Card, error) {
	return m.updateResult, m.updateErr
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

// duplicateCardMockRepo embeds cardMockRepo and overrides the two methods
// exercised by the duplicate-front branch: Create returns ErrCardDuplicateFront
// and FindByCardgroupAndFront returns the configured existing card.
type duplicateCardMockRepo struct {
	cardMockRepo
	existingCard *domain.Card
}

func (m *duplicateCardMockRepo) Create(_ context.Context, _ *domain.Card) error {
	return repository.ErrCardDuplicateFront
}

func (m *duplicateCardMockRepo) FindByCardgroupAndFront(_ context.Context, _, _ string) (*domain.Card, error) {
	return m.existingCard, nil
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
	cardUC := usecase.NewCardUsecaseWithTx(cardRepo, cgRepo, tx, nil, nil, newDiscardLogger())
	r := resolver.NewResolver(nil, nil, cardUC, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

func newLearnSrv(cardRepo *cardMockRepo, cgRepo *cardMockCGRepo) *handler.Server {
	learnUC := usecase.NewLearnUsecase(
		cardRepo,
		cgRepo,
		service.NewOrderingPolicy(),
		func() *rand.Rand { return rand.New(rand.NewSource(1)) },
		20,
		100,
		nil,
		newDiscardLogger(),
	)
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, nil, nil, learnUC, nil)
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

func TestResolver_LearnNextDueCards_ReturnsDueCards(t *testing.T) {
	t.Parallel()

	c1 := &domain.Card{
		ID:          "c1",
		CardgroupID: "cg1",
		Front:       "front",
		Back:        "back",
	}
	cardRepo := &cardMockRepo{
		findDueRows: []domain.DueCard{
			{Card: c1, State: domain.FSRSStateNew, Due: c1.CreatedAt},
		},
	}
	srv := newLearnSrv(
		cardRepo,
		&cardMockCGRepo{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)

	body := `{"query":"query($cardgroupId: ID!, $limit: Int!) { learnNextDueCards(cardgroupId: $cardgroupId, limit: $limit) { id front back cardgroupId } }","variables":{"cardgroupId":"cg1","limit":5}}`
	resp := gqlRequest(t, srv, authedCtx("u1"), body)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	cards, ok := data["learnNextDueCards"].([]any)
	if !ok || len(cards) != 1 {
		t.Fatalf("expected one card, got %T %v", data["learnNextDueCards"], data["learnNextDueCards"])
	}
	card, _ := cards[0].(map[string]any)
	if card["id"] != "c1" || card["front"] != "front" {
		t.Fatalf("unexpected card payload: %v", card)
	}
	if cardRepo.findDueLimit != 5 {
		t.Fatalf("expected limit 5, got %d", cardRepo.findDueLimit)
	}
}

func TestResolver_LearnNextDueCards_Anonymous(t *testing.T) {
	t.Parallel()

	srv := newLearnSrv(&cardMockRepo{}, &cardMockCGRepo{})

	body := `{"query":"query { learnNextDueCards(cardgroupId: \"cg1\") { id } }"}`
	resp := gqlRequest(t, srv, context.Background(), body)

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

func TestResolver_LearnNextDueCards_EmptyListIsNormal(t *testing.T) {
	t.Parallel()

	srv := newLearnSrv(
		&cardMockRepo{findDueRows: []domain.DueCard{}},
		&cardMockCGRepo{findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"}},
	)

	body := `{"query":"query { learnNextDueCards(cardgroupId: \"cg1\") { id } }"}`
	resp := gqlRequest(t, srv, authedCtx("u1"), body)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	cards, _ := data["learnNextDueCards"].([]any)
	if len(cards) != 0 {
		t.Fatalf("expected empty card list, got %v", data["learnNextDueCards"])
	}
}

// TestResolver_CreateCard_DuplicateFront_ReturnsCardDuplicateFrontError verifies
// that when the usecase returns a duplicate-front outcome, the resolver maps it
// to the CardDuplicateFrontError union variant with the correct fields populated.
func TestResolver_CreateCard_DuplicateFront_ReturnsCardDuplicateFrontError(t *testing.T) {
	t.Parallel()

	// cardgroup is owned by the authenticated user so authorization passes.
	cgRepo := &cardMockCGRepo{
		findResult: &domain.Cardgroup{ID: "cg1", OwnerID: "u1"},
	}
	// The mock repo causes Create to return ErrCardDuplicateFront and then
	// FindByCardgroupAndFront to return the existing card identified by "ex-1".
	cardRepo := &duplicateCardMockRepo{
		existingCard: &domain.Card{
			ID:          "ex-1",
			CardgroupID: "cg1",
			Front:       "Question",
			Back:        "existing back",
		},
	}
	srv := newCardSrv(cardRepo, cgRepo, cardFakeTx())

	mutation := map[string]any{
		"query": `mutation($input: NewCardInput!) {
			createCard(input: $input) {
				__typename
				... on CardDuplicateFrontError {
					message
					existingCardId
					existingBack
				}
			}
		}`,
		"variables": map[string]any{
			"input": map[string]any{
				"cardgroupId": "cg1",
				"front":       "Question",
				"back":        "Answer",
			},
		},
	}
	bodyBytes, _ := json.Marshal(mutation)
	resp := gqlRequest(t, srv, authedCtx("u1"), string(bodyBytes))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	result, _ := data["createCard"].(map[string]any)
	if result == nil {
		t.Fatalf("expected data.createCard, got nil; full response: %v", resp)
	}

	typeName, _ := result["__typename"].(string)
	if typeName != "CardDuplicateFrontError" {
		t.Fatalf("expected __typename=CardDuplicateFrontError, got %q", typeName)
	}
	existingCardID, _ := result["existingCardId"].(string)
	if existingCardID != "ex-1" {
		t.Fatalf("expected existingCardId=ex-1, got %q", existingCardID)
	}
	existingBack, _ := result["existingBack"].(string)
	if existingBack != "existing back" {
		t.Fatalf("expected existingBack=\"existing back\", got %q", existingBack)
	}
	message, _ := result["message"].(string)
	if message == "" {
		t.Fatalf("expected non-empty message, got empty string")
	}
}

// ---------------------------------------------------------------------------
// TestResolver_UpdateCard_* — standard four-case coverage for the UpdateCard
// union mutation (UpdateCardResult = UpdateCardSuccess | InputValidationError).
// ---------------------------------------------------------------------------

// newUpdateCardSrv builds a gqlgen Server backed by a CardUsecase wired with
// the supplied card repo and cardgroup repo for authorization.
func newUpdateCardSrv(cardRepo usecase.CardRepository, cgRepo usecase.CardgroupRepositoryForCard) *handler.Server {
	cardUC := usecase.NewCardUsecaseWithTx(cardRepo, cgRepo, cardFakeTx(), nil, nil, newDiscardLogger())
	r := resolver.NewResolver(nil, nil, cardUC, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// updateCardMutation returns a JSON-encoded GraphQL mutation body for
// updateCard, selecting across both union variants.
func updateCardMutation(id, front, back string) string {
	b, _ := json.Marshal(map[string]any{
		"query": `mutation($id: ID!, $input: UpdateCardInput!) {
			updateCard(id: $id, input: $input) {
				__typename
				... on UpdateCardSuccess { card { id front back } }
				... on InputValidationError { field message }
			}
		}`,
		"variables": map[string]any{
			"id": id,
			"input": map[string]any{
				"front": front,
				"back":  back,
			},
		},
	})
	return string(b)
}

// TestResolver_UpdateCard_HappyPath verifies that a successful update returns
// the UpdateCardSuccess union variant with the updated card.
func TestResolver_UpdateCard_HappyPath(t *testing.T) {
	t.Parallel()

	updatedCard := &domain.Card{ID: "c-1", CardgroupID: "cg-1", Front: "NewFront", Back: "NewBack"}
	cardRepo := &cardMockRepo{
		findByIDResult: &domain.Card{ID: "c-1", CardgroupID: "cg-1", Front: "OldFront", Back: "OldBack"},
		updateResult:   updatedCard,
	}
	cgRepo := &cardMockCGRepo{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"},
	}
	srv := newUpdateCardSrv(cardRepo, cgRepo)

	resp := gqlRequest(t, srv, authedCtx("u-1"), updateCardMutation("c-1", "NewFront", "NewBack"))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateCard"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.updateCard, got nil; response: %v", resp)
	}
	if payload["__typename"] != "UpdateCardSuccess" {
		t.Fatalf("expected __typename=UpdateCardSuccess, got %v; response: %v", payload["__typename"], resp)
	}
	card, _ := payload["card"].(map[string]any)
	if card == nil {
		t.Fatalf("expected card in success payload, got nil; response: %v", resp)
	}
	if card["id"] != "c-1" {
		t.Fatalf("expected card.id=c-1, got %v", card["id"])
	}
	if card["front"] != "NewFront" {
		t.Fatalf("expected card.front=NewFront, got %v", card["front"])
	}
}

// TestResolver_UpdateCard_InputValidation verifies that patching front with an
// empty string is surfaced as the InputValidationError union variant
// (errors as data), not as a GraphQL protocol error.
func TestResolver_UpdateCard_InputValidation(t *testing.T) {
	t.Parallel()

	cardRepo := &cardMockRepo{
		findByIDResult: &domain.Card{ID: "c-1", CardgroupID: "cg-1", Front: "OldFront", Back: "OldBack"},
	}
	cgRepo := &cardMockCGRepo{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"},
	}
	srv := newUpdateCardSrv(cardRepo, cgRepo)

	// An empty front fails card.Validate() → InputValidationError.
	resp := gqlRequest(t, srv, authedCtx("u-1"), updateCardMutation("c-1", "", "SomeBack"))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors (validation should come as data): %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateCard"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.updateCard, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v; response: %v", payload["__typename"], resp)
	}
	if payload["field"] == nil || payload["field"] == "" {
		t.Fatalf("expected non-empty field in InputValidationError, got %v", payload["field"])
	}
}

// TestResolver_UpdateCard_Unauthenticated verifies that an anonymous request
// is rejected with UNAUTHENTICATED via gqlerr.FromUsecaseError.
func TestResolver_UpdateCard_Unauthenticated(t *testing.T) {
	t.Parallel()

	cardRepo := &cardMockRepo{}
	cgRepo := &cardMockCGRepo{}
	srv := newUpdateCardSrv(cardRepo, cgRepo)

	resp := gqlRequest(t, srv, context.Background(), updateCardMutation("c-1", "Front", "Back"))

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// TestResolver_UpdateCard_NilVariant_ReturnsInternal covers the defensive guard
// in the resolver where the usecase returns an UpdateCardOutcome with both Card
// and Validation nil (a bug shape). This is triggered by having cardRepo.Update
// return nil, nil — the usecase then returns UpdateCardOutcome{Card: nil} with
// nil error, hitting the resolver's INTERNAL guard.
func TestResolver_UpdateCard_NilVariant_ReturnsInternal(t *testing.T) {
	t.Parallel()

	cardRepo := &cardMockRepo{
		findByIDResult: &domain.Card{ID: "c-1", CardgroupID: "cg-1", Front: "OldFront", Back: "OldBack"},
		updateResult:   nil, // triggers nil-variant path
	}
	cgRepo := &cardMockCGRepo{
		findResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"},
	}
	srv := newUpdateCardSrv(cardRepo, cgRepo)

	resp := gqlRequest(t, srv, authedCtx("u-1"), updateCardMutation("c-1", "NewFront", "NewBack"))

	code := errCode(t, resp)
	if code != "INTERNAL" {
		t.Fatalf("expected INTERNAL, got %q; response: %v", code, resp)
	}
}
