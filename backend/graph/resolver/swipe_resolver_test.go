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

// ---------------------------------------------------------------------------
// Mock repositories for SwipeUsecase
// ---------------------------------------------------------------------------

// swipeCardRepo satisfies usecase.CardRepoForSwipe.
type swipeCardRepo struct {
	findByIDTxResult *domain.Card
	findByIDTxErr    error
}

func (m *swipeCardRepo) FindByIDTx(_ context.Context, _ *gorm.DB, _ string) (*domain.Card, error) {
	return m.findByIDTxResult, m.findByIDTxErr
}

// swipeCGRepo satisfies usecase.CardgroupRepoForSwipe.
type swipeCGRepo struct {
	findByIDResult *domain.Cardgroup
	findByIDErr    error
}

func (m *swipeCGRepo) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.findByIDResult, m.findByIDErr
}

// swipeRecordRepo satisfies usecase.SwipeRecordRepoForSwipe.
type swipeRecordRepo struct {
	createTxErr      error
	listRecentResult []*domain.SwipeRecord
	listRecentErr    error
}

func (m *swipeRecordRepo) CreateTx(_ context.Context, _ *gorm.DB, _ *domain.SwipeRecord) error {
	return m.createTxErr
}

func (m *swipeRecordRepo) ListRecentByUser(_ context.Context, _ string, _ int) ([]*domain.SwipeRecord, error) {
	return m.listRecentResult, m.listRecentErr
}

// userCardFSRSRepo satisfies usecase.UserCardFSRSRepoForSwipe.
type userCardFSRSRepo struct {
	findByIDsResult   map[string]*domain.UserCardFSRS
	findByIDsErr      error
	findByIDsTxResult map[string]*domain.UserCardFSRS
	findByIDsTxErr    error
	upsertTxErr       error
}

func (m *userCardFSRSRepo) UpsertTx(_ context.Context, _ *gorm.DB, _ *domain.UserCardFSRS) error {
	return m.upsertTxErr
}

func (m *userCardFSRSRepo) FindByUserAndCardIDs(_ context.Context, _ string, _ []string) (map[string]*domain.UserCardFSRS, error) {
	return m.findByIDsResult, m.findByIDsErr
}

func (m *userCardFSRSRepo) FindByUserAndCardIDsTx(_ context.Context, _ *gorm.DB, _ string, _ []string) (map[string]*domain.UserCardFSRS, error) {
	return m.findByIDsTxResult, m.findByIDsTxErr
}

// ---------------------------------------------------------------------------
// Server construction helpers
// ---------------------------------------------------------------------------

// swipeFakeTx returns a txRunner stub that executes fn with a nil *gorm.DB.
// The swipe mock repos ignore the tx argument.
func swipeFakeTx() func(context.Context, func(*gorm.DB) error) error {
	return func(_ context.Context, fn func(*gorm.DB) error) error {
		return fn(nil)
	}
}

// newSwipeSrv builds a gqlgen Server backed by a SwipeUsecase wired with the
// supplied mock repositories and an in-process tx runner.
func newSwipeSrv(
	cardRepo usecase.CardRepoForSwipe,
	cgRepo usecase.CardgroupRepoForSwipe,
	swipeRepo usecase.SwipeRecordRepoForSwipe,
	userFSRSRepo usecase.UserCardFSRSRepoForSwipe,
) *handler.Server {
	swipeUC := usecase.NewSwipeUsecaseWithTx(
		cardRepo,
		cgRepo,
		swipeRepo,
		nil, // scheduler — nil uses default FSRSScheduler
		swipeFakeTx(),
		userFSRSRepo,
		newDiscardLogger(),
	)
	r := resolver.NewResolver(nil, nil, nil, swipeUC, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// handleSwipeMutation returns a JSON-encoded GraphQL mutation body for
// handleSwipe, selecting across both union variants.
func handleSwipeMutation(cardID, cardgroupID string, mode int) string {
	b, _ := json.Marshal(map[string]any{
		"query": `mutation($input: HandleSwipeInput!) {
			handleSwipe(input: $input) {
				__typename
				... on HandleSwipeSuccess {
					response {
						performanceMode
						metrics { successRate avgDifficulty retentionRate studyStreak lapseRate reviewCount }
					}
				}
				... on InputValidationError { field message }
			}
		}`,
		"variables": map[string]any{
			"input": map[string]any{
				"cardId":      cardID,
				"cardgroupId": cardgroupID,
				"mode":        mode,
			},
		},
	})
	return string(b)
}

// ---------------------------------------------------------------------------
// TestResolver_HandleSwipe_* — standard four-case coverage
// ---------------------------------------------------------------------------

// TestResolver_HandleSwipe_HappyPath verifies that a successful swipe returns
// the HandleSwipeSuccess union variant with the swipe response payload.
func TestResolver_HandleSwipe_HappyPath(t *testing.T) {
	t.Parallel()

	card := &domain.Card{ID: "c-1", CardgroupID: "cg-1", Front: "Q", Back: "A"}

	cardRepo := &swipeCardRepo{
		findByIDTxResult: card,
	}
	cgRepo := &swipeCGRepo{
		findByIDResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"},
	}
	swipeRepo := &swipeRecordRepo{
		listRecentResult: []*domain.SwipeRecord{},
	}
	fsrsRepo := &userCardFSRSRepo{
		findByIDsTxResult: map[string]*domain.UserCardFSRS{}, // no prior state → new card
	}

	srv := newSwipeSrv(cardRepo, cgRepo, swipeRepo, fsrsRepo)

	// Mode 1 = Again — a valid swipe mode.
	resp := gqlRequest(t, srv, authedCtx("u-1"), handleSwipeMutation("c-1", "cg-1", 1))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["handleSwipe"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.handleSwipe, got nil; response: %v", resp)
	}
	if payload["__typename"] != "HandleSwipeSuccess" {
		t.Fatalf("expected __typename=HandleSwipeSuccess, got %v; response: %v", payload["__typename"], resp)
	}
	response, _ := payload["response"].(map[string]any)
	if response == nil {
		t.Fatalf("expected response in HandleSwipeSuccess, got nil; response: %v", resp)
	}
}

// TestResolver_HandleSwipe_InputValidation verifies that an invalid swipe mode
// is surfaced as the InputValidationError union variant (errors as data), not
// as a GraphQL protocol error. The mode validation fires before any repo calls.
func TestResolver_HandleSwipe_InputValidation(t *testing.T) {
	t.Parallel()

	// Repos can be minimal stubs — mode validation fires before any repo access.
	cardRepo := &swipeCardRepo{}
	cgRepo := &swipeCGRepo{
		findByIDResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"},
	}
	swipeRepo := &swipeRecordRepo{}
	fsrsRepo := &userCardFSRSRepo{}

	srv := newSwipeSrv(cardRepo, cgRepo, swipeRepo, fsrsRepo)

	// Mode 999 is not a valid swipe mode → InputValidationError{field:"mode"}.
	resp := gqlRequest(t, srv, authedCtx("u-1"), handleSwipeMutation("c-1", "cg-1", 999))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors (validation should come as data): %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["handleSwipe"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.handleSwipe, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v; response: %v", payload["__typename"], resp)
	}
	if payload["field"] != "mode" {
		t.Fatalf("expected field=mode, got %v", payload["field"])
	}
	if payload["message"] == nil || payload["message"] == "" {
		t.Fatalf("expected non-empty message in InputValidationError, got %v", payload["message"])
	}
}

// TestResolver_HandleSwipe_Unauthenticated verifies that an anonymous request
// is rejected with UNAUTHENTICATED via gqlerr.FromUsecaseError.
func TestResolver_HandleSwipe_Unauthenticated(t *testing.T) {
	t.Parallel()

	cardRepo := &swipeCardRepo{}
	cgRepo := &swipeCGRepo{}
	swipeRepo := &swipeRecordRepo{}
	fsrsRepo := &userCardFSRSRepo{}

	srv := newSwipeSrv(cardRepo, cgRepo, swipeRepo, fsrsRepo)

	resp := gqlRequest(t, srv, context.Background(), handleSwipeMutation("c-1", "cg-1", 1))

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// TestResolver_HandleSwipe_InfrastructureError_ReturnsInternal verifies that an
// infrastructure error from HandleSwipe (e.g. a nil tx runner) maps to INTERNAL
// via gqlerr.FromUsecaseError. The nil-variant guard at the resolver
// (if outcome.Swipe == nil) is structurally unreachable through the real usecase
// — HandleSwipe never returns a zero-value HandleSwipeOutcome alongside nil error
// — so the guard itself has no test, only this nearest reachable proxy for the
// INTERNAL mapping.
func TestResolver_HandleSwipe_InfrastructureError_ReturnsInternal(t *testing.T) {
	t.Parallel()

	// Build the SwipeUsecase directly with a nil tx to trigger the
	// "transaction runner is not configured" INTERNAL error path.
	swipeUC := usecase.NewSwipeUsecaseWithTx(
		&swipeCardRepo{},
		&swipeCGRepo{
			findByIDResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"},
		},
		&swipeRecordRepo{},
		nil,
		nil, // nil tx runner → INTERNAL
		&userCardFSRSRepo{},
		newDiscardLogger(),
	)
	r := resolver.NewResolver(nil, nil, nil, swipeUC, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	// Mode 1 is valid — passes mode validation, reaches tx-runner check.
	resp := gqlRequest(t, srv, authedCtx("u-1"), handleSwipeMutation("c-1", "cg-1", 1))

	code := errCode(t, resp)
	if code != "INTERNAL" {
		t.Fatalf("expected INTERNAL, got %q; response: %v", code, resp)
	}
}

// TestSwipeResponseSchema_DoesNotExposeNextCards is a regression guard for
// issue #229. The fix dropped the `nextCards` field from `SwipeResponse`
// because the field carried a freshly-shuffled queue snapshot that the
// client also tracked, creating a dual-source-of-truth bug. If a future
// change re-introduces the field on the SwipeResponse type, this test
// fails immediately at unit-test time.
//
// The test uses gqlgen's parsed schema (not a runtime mutation) because:
//   - The parsed schema is the canonical wire contract.
//   - A runtime mutation requires resolver + repository mocks; if the
//     resolver panics on incomplete mocks, the recovered error masks
//     the schema-level rejection signal.
//   - Schema introspection runs in microseconds and is dependency-free.
func TestSwipeResponseSchema_DoesNotExposeNextCards(t *testing.T) {
	t.Parallel()

	schema := generated.NewExecutableSchema(generated.Config{}).Schema()
	swipeResponse, ok := schema.Types["SwipeResponse"]
	if !ok {
		t.Fatal("SwipeResponse type missing from schema")
	}
	for _, field := range swipeResponse.Fields {
		if field.Name == "nextCards" {
			t.Fatalf("SwipeResponse must not expose `nextCards` field (issue #229 regression); current fields: %v", swipeResponse.Fields)
		}
	}
}

// TestResolver_HandleSwipe_CardNotFound_InputValidation verifies that when the
// card repo returns ErrNotFound inside the transaction, the resolver surfaces
// the InputValidationError union variant with field "cardId" (errors as data),
// not a GraphQL protocol error.
func TestResolver_HandleSwipe_CardNotFound_InputValidation(t *testing.T) {
	t.Parallel()

	cardRepo := &swipeCardRepo{
		findByIDTxErr: repository.ErrNotFound,
	}
	cgRepo := &swipeCGRepo{
		findByIDResult: &domain.Cardgroup{ID: "cg-1", OwnerID: "u-1"},
	}
	swipeRepo := &swipeRecordRepo{}
	fsrsRepo := &userCardFSRSRepo{}

	srv := newSwipeSrv(cardRepo, cgRepo, swipeRepo, fsrsRepo)

	// Mode 1 = Again — passes mode validation, reaches the card lookup.
	resp := gqlRequest(t, srv, authedCtx("u-1"), handleSwipeMutation("c-missing", "cg-1", 1))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors (validation should come as data): %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["handleSwipe"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.handleSwipe, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v; response: %v", payload["__typename"], resp)
	}
	if payload["field"] != "cardId" {
		t.Fatalf("expected field=cardId, got %v", payload["field"])
	}
	if payload["message"] != "card not found" {
		t.Fatalf("expected message=%q, got %v", "card not found", payload["message"])
	}
}
