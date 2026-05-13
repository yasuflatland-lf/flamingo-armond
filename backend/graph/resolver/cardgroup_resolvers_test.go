package resolver_test

import (
	"context"
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
}

func (m *mockCardgroupRepoForResolver) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.findByIDResult, m.findByIDErr
}

func (m *mockCardgroupRepoForResolver) FindByOwner(_ context.Context, _ string) ([]*domain.Cardgroup, error) {
	return nil, nil
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
	return nil, nil
}

func (m *mockCardgroupRepoForResolver) Delete(_ context.Context, _ string) error {
	return nil
}

// newCardgroupSrv builds a gqlgen handler.Server backed by a real
// CardgroupUsecase wired to the supplied mock repository.
func newCardgroupSrv(repo usecase.CardgroupRepository) *handler.Server {
	cgUC := usecase.NewCardgroupUsecase(repo)
	r := resolver.NewResolver(nil, cgUC, nil, nil, nil, nil, nil, nil, nil, nil)
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
		{ID: "cg1", OwnerID: "u1", Name: "Alpha"},
		{ID: "cg2", OwnerID: "u1", Name: "Beta"},
		// The third row is the "+1" fetch the usecase requests so it can
		// detect hasNextPage; the resolver response should NOT include it.
		{ID: "cg3", OwnerID: "u1", Name: "Gamma"},
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
// helpers.go — toCardgroupConnectionModel / toUsecaseCardgroupOrderBy
// ---------------------------------------------------------------------------
//
// These helpers live in package resolver but are package-private. The tests
// below exercise them indirectly via the MyCardgroupsConnection resolver:
//
// - toCardgroupConnectionModel non-empty path: covered by
//   TestResolver_MyCardgroupsConnection_Authenticated_DelegatesToUsecase.
// - toUsecaseCardgroupOrderBy with explicit value: covered by
//   TestResolver_MyCardgroupsConnection_OrderByName_PassesThrough below.
// - toUsecaseCardgroupOrderBy with nil (default UPDATED_AT) and the empty
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
// the schema-level orderBy=NAME enum is translated by toUsecaseCardgroupOrderBy
// and reaches the repository as the corresponding usecase enum. The mock
// captures the orderBy passed to FindPageByOwner.
func TestResolver_MyCardgroupsConnection_OrderByName_PassesThrough(t *testing.T) {
	t.Parallel()

	cgs := []*domain.Cardgroup{
		{ID: "cg-a", OwnerID: "u1", Name: "Apple"},
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
