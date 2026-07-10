package resolver_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

// ---------------------------------------------------------------------------
// mockCardgroupRepoForResolver — test double for usecase.CardgroupRepository
// used by the MyCardgroupsConnection resolver tests. Mirrors the test double
// living in internal/usecase/cardgroup_test.go but kept local because the
// usecase package's mock is not exported.
// ---------------------------------------------------------------------------

type mockCardgroupRepoForResolver struct {
	// FindByID is exercised when a resolver test passes after/before cursors.
	findByIDResult *domain.Cardgroup
	findByIDErr    error

	// FindPageByOwner / CountByOwner — the two methods MyCardgroupsConnection
	// hits on the happy path.
	findPageResult []*domain.Cardgroup
	findPageErr    error
	countResult    int64
	countErr       error

	// updateResult / updateErr control the return value of Update.
	// Used by the TestResolver_UpdateCardgroup_* tests.
	updateResult *domain.Cardgroup
	updateErr    error
}

func (m *mockCardgroupRepoForResolver) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.findByIDResult, m.findByIDErr
}

func (m *mockCardgroupRepoForResolver) FindPageByOwner(
	_ context.Context,
	_ string,
	_, _ *repository.CardgroupCursor,
	_, _ int,
	_ repository.CardgroupOrderBy,
	_ repository.SortOrder,
	_ *string,
) ([]*domain.Cardgroup, error) {
	return m.findPageResult, m.findPageErr
}

func (m *mockCardgroupRepoForResolver) CountByOwner(_ context.Context, _ string, _ *string) (int64, error) {
	return m.countResult, m.countErr
}

func (m *mockCardgroupRepoForResolver) Create(_ context.Context, _ *domain.Cardgroup) error {
	return nil
}

func (m *mockCardgroupRepoForResolver) Update(_ context.Context, _ string, _ repository.CardgroupUpdate) (*domain.Cardgroup, error) {
	return m.updateResult, m.updateErr
}

func (m *mockCardgroupRepoForResolver) Delete(_ context.Context, _ string) error {
	return nil
}

// stubAdminCheckerForResolver satisfies usecase.AdminChecker for resolver-level
// unit tests. Most tests pass isAdmin: true to bypass the cardgroup-limit guard
// and preserve prior behavior; the limit-reached test passes isAdmin: false.
type stubAdminCheckerForResolver struct {
	isAdmin bool
	err     error
}

func (s stubAdminCheckerForResolver) IsAdmin(_ context.Context, _ string) (bool, error) {
	return s.isAdmin, s.err
}

// newCardgroupSrv builds a gqlgen handler.Server backed by a real
// CardgroupUsecase wired to the supplied mock repository. The admin stub
// returns isAdmin: true so the cardgroup-limit guard is bypassed for all
// tests that do not specifically exercise the limit path.
func newCardgroupSrv(repo usecase.CardgroupRepository) *handler.Server {
	return newCardgroupSrvWithAdmin(repo, stubAdminCheckerForResolver{isAdmin: true})
}

// newCardgroupSrvWithAdmin builds a gqlgen handler.Server allowing the caller
// to supply a custom AdminChecker stub. Use this when a test needs to exercise
// the cardgroup-limit code path (isAdmin: false).
func newCardgroupSrvWithAdmin(repo usecase.CardgroupRepository, admin usecase.AdminChecker) *handler.Server {
	cgUC := usecase.NewCardgroupUsecase(repo, admin, newDiscardLogger())
	r := resolver.NewResolver(nil, cgUC, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ---------------------------------------------------------------------------
// MyCardgroupsConnection resolver tests
// ---------------------------------------------------------------------------

const myCardgroupsConnectionQuery = `{"query":"{ myCardgroupsConnection(first: 2) { totalCount edges { cursor node { id name } } pageInfo { hasNextPage hasPreviousPage startCursor endCursor } } }"}`

// TestResolver_MyCardgroupsConnection_Authenticated_DelegatesToUsecase
// verifies the resolver returns the usecase output as a model.CardgroupConnection.
func TestResolver_MyCardgroupsConnection_Authenticated_DelegatesToUsecase(t *testing.T) {
	t.Parallel()

	cgs := []*domain.Cardgroup{
		{ID: domain.CardgroupID("cg1"), OwnerID: "u1", Name: "Alpha"},
		{ID: domain.CardgroupID("cg2"), OwnerID: "u1", Name: "Beta"},
		// The third row is the "+1" fetch the usecase requests so it can
		// detect hasNextPage; the resolver response should NOT include it.
		{ID: domain.CardgroupID("cg3"), OwnerID: "u1", Name: "Gamma"},
	}
	repo := &mockCardgroupRepoForResolver{
		findPageResult: cgs,
		countResult:    3,
	}
	srv := newCardgroupSrv(repo)
	resp := gqlRequest(t, srv, authedCtx("u1"), myCardgroupsConnectionQuery)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	conn, _ := data["myCardgroupsConnection"].(map[string]any)
	if conn == nil {
		t.Fatalf("expected data.myCardgroupsConnection, got nil; response: %v", resp)
	}

	totalCount, _ := conn["totalCount"].(float64)
	if int(totalCount) != 3 {
		t.Fatalf("expected totalCount=3, got %v", totalCount)
	}
	edges, _ := conn["edges"].([]any)
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges (third row trimmed), got %d", len(edges))
	}
	wantCursorCG1 := cursor.Encode("cg1")
	wantCursorCG2 := cursor.Encode("cg2")

	first, _ := edges[0].(map[string]any)
	if first["cursor"] != wantCursorCG1 {
		t.Fatalf("expected first edge cursor=%q, got %v", wantCursorCG1, first["cursor"])
	}
	node, _ := first["node"].(map[string]any)
	if node["name"] != "Alpha" {
		t.Fatalf("expected first node.name=Alpha, got %v", node["name"])
	}

	pageInfo, _ := conn["pageInfo"].(map[string]any)
	if pageInfo["hasNextPage"] != true {
		t.Fatalf("expected hasNextPage=true (overflow row), got %v", pageInfo["hasNextPage"])
	}
	if pageInfo["hasPreviousPage"] != false {
		t.Fatalf("expected hasPreviousPage=false on page 1, got %v", pageInfo["hasPreviousPage"])
	}
	if pageInfo["startCursor"] != wantCursorCG1 {
		t.Fatalf("expected startCursor=%q, got %v", wantCursorCG1, pageInfo["startCursor"])
	}
	if pageInfo["endCursor"] != wantCursorCG2 {
		t.Fatalf("expected endCursor=%q, got %v", wantCursorCG2, pageInfo["endCursor"])
	}
}

// TestResolver_MyCardgroupsConnection_Unauthenticated_ReturnsUnauthenticated
// verifies an anonymous request surfaces extensions.code == UNAUTHENTICATED.
func TestResolver_MyCardgroupsConnection_Unauthenticated_ReturnsUnauthenticated(t *testing.T) {
	t.Parallel()

	srv := newCardgroupSrv(&mockCardgroupRepoForResolver{})
	resp := gqlRequest(t, srv, context.Background(), myCardgroupsConnectionQuery)

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// TestResolver_MyCardgroupsConnection_UsecaseError_PropagatesBadUserInput
// verifies the resolver propagates a typed GraphQL error from the usecase
// rather than swallowing it. Triggering it: pass after AND before so the
// usecase rejects with BAD_USER_INPUT before reaching the repository.
func TestResolver_MyCardgroupsConnection_UsecaseError_PropagatesBadUserInput(t *testing.T) {
	t.Parallel()

	srv := newCardgroupSrv(&mockCardgroupRepoForResolver{})
	body := `{"query":"{ myCardgroupsConnection(first: 2, after: \"cg-a\", before: \"cg-b\") { totalCount } }"}`
	resp := gqlRequest(t, srv, authedCtx("u1"), body)

	ext := errExtensions(t, resp)
	code, _ := ext["code"].(string)
	if code != "BAD_USER_INPUT" {
		t.Fatalf("expected BAD_USER_INPUT, got %q", code)
	}
	field, _ := ext["field"].(string)
	if field != "after" {
		t.Fatalf("expected extensions.field=after, got %q", field)
	}
}

// TestResolver_MyCardgroupsConnection_RepoCountError_BecomesInternal
// verifies that an unexpected repository error from CountByOwner is mapped
// to extensions.code == INTERNAL.
func TestResolver_MyCardgroupsConnection_RepoCountError_BecomesInternal(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepoForResolver{countErr: errors.New("db died")}
	srv := newCardgroupSrv(repo)
	resp := gqlRequest(t, srv, authedCtx("u1"), myCardgroupsConnectionQuery)

	code := errCode(t, resp)
	if code != "INTERNAL" {
		t.Fatalf("expected INTERNAL, got %q", code)
	}
}

// ---------------------------------------------------------------------------
// helpers.go — toCardgroupConnectionModel / toUsecaseOrderBy
// ---------------------------------------------------------------------------
//
// These helpers live in package resolver but are package-private. The tests
// below exercise them indirectly via the MyCardgroupsConnection resolver:
//
// - toCardgroupConnectionModel non-empty path: covered by
//   TestResolver_MyCardgroupsConnection_Authenticated_DelegatesToUsecase.
// - toUsecaseOrderBy with an explicit CardgroupOrderBy value: covered by
//   TestResolver_MyCardgroupsConnection_OrderByName_PassesThrough below.
// - toUsecaseOrderBy with nil (default UPDATED_AT) and the empty
//   connection branch: exercised by the empty-result test below.

// TestResolver_MyCardgroupsConnection_Empty_ReturnsEmptyEdges verifies that an
// empty page result returns edges: [] with no startCursor/endCursor. This
// exercises the toCardgroupConnectionModel branch where len(out.Cardgroups)==0
// so StartCur/EndCur stay empty and nilIfEmpty maps them to JSON null.
func TestResolver_MyCardgroupsConnection_Empty_ReturnsEmptyEdges(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepoForResolver{
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	srv := newCardgroupSrv(repo)
	resp := gqlRequest(t, srv, authedCtx("u1"), myCardgroupsConnectionQuery)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	conn, _ := data["myCardgroupsConnection"].(map[string]any)
	if conn == nil {
		t.Fatalf("expected data.myCardgroupsConnection, got nil")
	}
	totalCount, _ := conn["totalCount"].(float64)
	if int(totalCount) != 0 {
		t.Fatalf("expected totalCount=0, got %v", totalCount)
	}
	edges, _ := conn["edges"].([]any)
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(edges))
	}
	pageInfo, _ := conn["pageInfo"].(map[string]any)
	if pageInfo["startCursor"] != nil {
		t.Fatalf("expected startCursor=nil for empty connection, got %v", pageInfo["startCursor"])
	}
	if pageInfo["endCursor"] != nil {
		t.Fatalf("expected endCursor=nil for empty connection, got %v", pageInfo["endCursor"])
	}
}

// TestResolver_MyCardgroupsConnection_OrderByName_PassesThrough verifies that
// the schema-level orderBy=NAME enum is translated by toUsecaseOrderBy
// and reaches the repository as the corresponding usecase enum. The mock
// captures the orderBy passed to FindPageByOwner.
func TestResolver_MyCardgroupsConnection_OrderByName_PassesThrough(t *testing.T) {
	t.Parallel()

	cgs := []*domain.Cardgroup{
		{ID: domain.CardgroupID("cg-a"), OwnerID: "u1", Name: "Apple"},
	}
	captureRepo := &capturingCardgroupRepo{
		mockCardgroupRepoForResolver: mockCardgroupRepoForResolver{
			findPageResult: cgs,
			countResult:    1,
		},
	}
	srv := newCardgroupSrv(captureRepo)
	body := `{"query":"{ myCardgroupsConnection(first: 5, orderBy: NAME, orderDirection: ASC) { totalCount edges { cursor } } }"}`
	resp := gqlRequest(t, srv, authedCtx("u1"), body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if captureRepo.findPageOrderBy != repository.CardgroupOrderByName {
		t.Fatalf("expected repository.CardgroupOrderByName at the repo seam, got %q", captureRepo.findPageOrderBy)
	}
	if captureRepo.findPageDir != repository.SortAsc {
		t.Fatalf("expected SortAsc at the repo seam, got %q", captureRepo.findPageDir)
	}
}

// capturingCardgroupRepo extends mockCardgroupRepoForResolver to capture the
// arguments FindPageByOwner is called with. Used by tests that need to
// assert on the orderBy/direction translation done by helpers.go.
type capturingCardgroupRepo struct {
	mockCardgroupRepoForResolver
	findPageOrderBy repository.CardgroupOrderBy
	findPageDir     repository.SortOrder
	findPageFirst   int
	findPageLast    int
	findPageAfter   *repository.CardgroupCursor
	findPageBefore  *repository.CardgroupCursor
}

func (c *capturingCardgroupRepo) FindPageByOwner(
	_ context.Context,
	_ string,
	after, before *repository.CardgroupCursor,
	first, last int,
	orderBy repository.CardgroupOrderBy,
	dir repository.SortOrder,
	_ *string,
) ([]*domain.Cardgroup, error) {
	c.findPageOrderBy = orderBy
	c.findPageDir = dir
	c.findPageFirst = first
	c.findPageLast = last
	c.findPageAfter = after
	c.findPageBefore = before
	return c.findPageResult, c.findPageErr
}

// ---------------------------------------------------------------------------
// CreateCardgroup resolver tests
// ---------------------------------------------------------------------------
//
// Note on the nil-variant (XOR-invariant) guard:
//
//   The resolver's CreateCardgroup method contains a defensive guard:
//
//     if outcome.Cardgroup == nil {
//         return nil, gqlerr.Internal(...)
//     }
//
//   CardgroupUsecase is a concrete struct (not an interface), so the resolver
//   cannot accept a mock implementation at the unit-test layer. The guard is
//   dead code: the usecase always sets outcome.Cardgroup on a nil-error path.
//   Its presence is a structural invariant, not a reachable branch. Integration
//   coverage for the happy path and error paths is provided by the tests in
//   backend/cmd/server/main_test.go (TestGraphQL_CreateCardgroup_*).

// createCardgroupBody returns a JSON-encoded mutation body that selects both
// union variants of CreateCardgroupResult. It is the CreateCardgroup analogue
// of setLastViewedMutation: each call site embeds its own name value but
// shares the fragment shape so variant coverage is consistent across tests.
// name must not contain double-quote characters.
func createCardgroupBody(name string) string {
	return `{"query":"mutation { createCardgroup(input: {name: \"` + name + `\"}) { __typename ... on CreateCardgroupSuccess { cardgroup { id name } } ... on InputValidationError { field message } } }"}`
}

// TestResolver_CreateCardgroup_HappyPath verifies that a successful create
// returns the CreateCardgroupSuccess union variant with a non-nil Cardgroup.
// This confirms the resolver reaches the success branch, not the nil guard.
func TestResolver_CreateCardgroup_HappyPath(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepoForResolver{}
	srv := newCardgroupSrv(repo)
	resp := gqlRequest(t, srv, authedCtx("u1"), createCardgroupBody("Test Group"))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.createCardgroup, got nil; response: %v", resp)
	}
	if payload["__typename"] != "CreateCardgroupSuccess" {
		t.Fatalf("expected CreateCardgroupSuccess, got %v; response: %v", payload["__typename"], resp)
	}
	cg, _ := payload["cardgroup"].(map[string]any)
	if cg == nil {
		t.Fatalf("expected cardgroup in success payload, got nil; response: %v", resp)
	}
	if cg["name"] != "Test Group" {
		t.Fatalf("expected cardgroup.name=Test Group, got %v", cg["name"])
	}
}

// TestResolver_CreateCardgroup_Unauthenticated verifies that an anonymous
// request is rejected with UNAUTHENTICATED via the usecase auth check.
func TestResolver_CreateCardgroup_Unauthenticated(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepoForResolver{}
	srv := newCardgroupSrv(repo)
	resp := gqlRequest(t, srv, context.Background(), createCardgroupBody("Test"))

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q; response: %v", code, resp)
	}
}

// TestResolver_CreateCardgroup_ValidationError_EmptyName verifies that an
// empty name is surfaced as the InputValidationError union variant (errors as
// data), not as a GraphQL protocol error.
func TestResolver_CreateCardgroup_ValidationError_EmptyName(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepoForResolver{}
	srv := newCardgroupSrv(repo)
	resp := gqlRequest(t, srv, authedCtx("u1"), createCardgroupBody(""))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors (validation should come as data): %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.createCardgroup, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected InputValidationError, got %v; response: %v", payload["__typename"], resp)
	}
	if payload["field"] != "name" {
		t.Fatalf("expected field=name, got %v; response: %v", payload["field"], resp)
	}
}

// createCardgroupBodyWithLimit returns a mutation body that selects all three
// union variants of CreateCardgroupResult, including CardgroupLimitReachedError.
func createCardgroupBodyWithLimit(name string) string {
	return `{"query":"mutation { createCardgroup(input: {name: \"` + name + `\"}) { __typename ... on CreateCardgroupSuccess { cardgroup { id name } } ... on InputValidationError { field message } ... on CardgroupLimitReachedError { message limit current } } }"}`
}

// TestResolver_CreateCardgroup_LimitReached verifies that a non-admin caller
// who already owns domain.GeneralUserCardgroupLimit (5) cardgroups receives the
// CardgroupLimitReachedError union variant with the correct Limit and Current
// fields rather than a GraphQL protocol error.
func TestResolver_CreateCardgroup_LimitReached(t *testing.T) {
	t.Parallel()

	// CountByOwner returns 5 — the non-admin caller is at the limit.
	repo := &mockCardgroupRepoForResolver{countResult: 5}
	// isAdmin: false so the limit check is not bypassed.
	srv := newCardgroupSrvWithAdmin(repo, stubAdminCheckerForResolver{isAdmin: false})
	resp := gqlRequest(t, srv, authedCtx("u1"), createCardgroupBodyWithLimit("Over Limit"))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected protocol errors (limit should come as data): %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.createCardgroup, got nil; response: %v", resp)
	}
	if payload["__typename"] != "CardgroupLimitReachedError" {
		t.Fatalf("expected CardgroupLimitReachedError, got %v; response: %v", payload["__typename"], resp)
	}
	limit, _ := payload["limit"].(float64)
	if int(limit) != 5 {
		t.Fatalf("expected limit=5, got %v; response: %v", limit, resp)
	}
	current, _ := payload["current"].(float64)
	if int(current) != 5 {
		t.Fatalf("expected current=5, got %v; response: %v", current, resp)
	}
}

// ---------------------------------------------------------------------------
// TestResolver_UpdateCardgroup_* — standard four-case coverage for the
// UpdateCardgroup union mutation
// (UpdateCardgroupResult = UpdateCardgroupSuccess | InputValidationError).
// ---------------------------------------------------------------------------

// updateCardgroupMutation returns a JSON-encoded GraphQL mutation body for
// updateCardgroup, selecting across both union variants.
func updateCardgroupMutation(id, name string) string {
	b, _ := json.Marshal(map[string]any{
		"query": `mutation($id: ID!, $input: UpdateCardgroupInput!) {
			updateCardgroup(id: $id, input: $input) {
				__typename
				... on UpdateCardgroupSuccess { cardgroup { id name } }
				... on InputValidationError { field message }
			}
		}`,
		"variables": map[string]any{
			"id": id,
			"input": map[string]any{
				"name": name,
			},
		},
	})
	return string(b)
}

// TestResolver_UpdateCardgroup_HappyPath verifies that a successful update
// returns the UpdateCardgroupSuccess union variant with the updated cardgroup.
func TestResolver_UpdateCardgroup_HappyPath(t *testing.T) {
	t.Parallel()

	updatedCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1", Name: "NewName"}
	repo := &mockCardgroupRepoForResolver{
		findByIDResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1", Name: "OldName"},
		updateResult:   updatedCG,
	}
	srv := newCardgroupSrv(repo)

	resp := gqlRequest(t, srv, authedCtx("u-1"), updateCardgroupMutation("cg-1", "NewName"))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.updateCardgroup, got nil; response: %v", resp)
	}
	if payload["__typename"] != "UpdateCardgroupSuccess" {
		t.Fatalf("expected __typename=UpdateCardgroupSuccess, got %v; response: %v", payload["__typename"], resp)
	}
	cg, _ := payload["cardgroup"].(map[string]any)
	if cg == nil {
		t.Fatalf("expected cardgroup in success payload, got nil; response: %v", resp)
	}
	if cg["id"] != "cg-1" {
		t.Fatalf("expected cardgroup.id=cg-1, got %v", cg["id"])
	}
	if cg["name"] != "NewName" {
		t.Fatalf("expected cardgroup.name=NewName, got %v", cg["name"])
	}
}

// TestResolver_UpdateCardgroup_InputValidation verifies that an empty name is
// surfaced as the InputValidationError union variant (errors as data), not as
// a GraphQL protocol error.
func TestResolver_UpdateCardgroup_InputValidation(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepoForResolver{
		findByIDResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1", Name: "OldName"},
	}
	srv := newCardgroupSrv(repo)

	// An empty name triggers domain.ErrCardgroupNameRequired → InputValidationError.
	resp := gqlRequest(t, srv, authedCtx("u-1"), updateCardgroupMutation("cg-1", ""))

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors (validation should come as data): %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateCardgroup"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.updateCardgroup, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v; response: %v", payload["__typename"], resp)
	}
	if payload["field"] == nil || payload["field"] == "" {
		t.Fatalf("expected non-empty field in InputValidationError, got %v", payload["field"])
	}
}

// TestResolver_UpdateCardgroup_Unauthenticated verifies that an anonymous
// request is rejected with UNAUTHENTICATED via gqlerr.FromUsecaseError.
func TestResolver_UpdateCardgroup_Unauthenticated(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepoForResolver{}
	srv := newCardgroupSrv(repo)

	resp := gqlRequest(t, srv, context.Background(), updateCardgroupMutation("cg-1", "SomeName"))

	code := errCode(t, resp)
	if code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

// TestResolver_UpdateCardgroup_NilVariant_ReturnsInternal covers the defensive
// guard in the resolver where the usecase returns an UpdateCardgroupOutcome with
// both Cardgroup and Validation nil (a bug shape). This is triggered by having
// repo.Update return nil, nil — the usecase returns
// UpdateCardgroupOutcome{Cardgroup: nil} with nil error, hitting the INTERNAL guard.
func TestResolver_UpdateCardgroup_NilVariant_ReturnsInternal(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepoForResolver{
		findByIDResult: &domain.Cardgroup{ID: domain.CardgroupID("cg-1"), OwnerID: "u-1", Name: "OldName"},
		updateResult:   nil, // triggers nil-variant path
	}
	srv := newCardgroupSrv(repo)

	resp := gqlRequest(t, srv, authedCtx("u-1"), updateCardgroupMutation("cg-1", "NewName"))

	code := errCode(t, resp)
	if code != "INTERNAL" {
		t.Fatalf("expected INTERNAL, got %q; response: %v", code, resp)
	}
}
