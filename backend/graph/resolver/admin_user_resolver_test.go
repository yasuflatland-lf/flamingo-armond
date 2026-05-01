package resolver_test

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/loader"
	"backend/internal/usecase"
)

// ---------------------------------------------------------------------------
// mockAdminUserUsecase — stub for AdminUserUsecase
// ---------------------------------------------------------------------------

type mockAdminUserUsecase struct {
	listResult      *usecase.AdminUserConnection
	listErr         error
	getResult       *domain.User
	getErr          error
	updateResult    *domain.User
	updateErr       error
	assignResult    *domain.User
	assignErr       error
	revokeResult    *domain.User
	revokeErr       error
	listRolesResult []*domain.Role
	listRolesErr    error
}

func (m *mockAdminUserUsecase) List(_ context.Context, _, _ *int, _, _, _ *string) (*usecase.AdminUserConnection, error) {
	return m.listResult, m.listErr
}
func (m *mockAdminUserUsecase) Get(_ context.Context, _ string) (*domain.User, error) {
	return m.getResult, m.getErr
}
func (m *mockAdminUserUsecase) Update(_ context.Context, _ string, _ usecase.AdminUpdateUserInput) (*domain.User, error) {
	return m.updateResult, m.updateErr
}
func (m *mockAdminUserUsecase) AssignRole(_ context.Context, _, _ string) (*domain.User, error) {
	return m.assignResult, m.assignErr
}
func (m *mockAdminUserUsecase) RevokeRole(_ context.Context, _, _ string) (*domain.User, error) {
	return m.revokeResult, m.revokeErr
}
func (m *mockAdminUserUsecase) ListRoles(_ context.Context) ([]*domain.Role, error) {
	return m.listRolesResult, m.listRolesErr
}

// mockRoleByUserIDRepo satisfies the minimal interface needed to build the
// RoleByUserID DataLoader.
type mockRoleByUserIDRepo struct {
	// byUserID maps user id → roles returned for that user.
	byUserID      map[string][]*domain.Role
	listCallCount int
}

func (m *mockRoleByUserIDRepo) ListByUserIDs(_ context.Context, ids []string) (map[string][]*domain.Role, error) {
	m.listCallCount++
	out := make(map[string][]*domain.Role, len(ids))
	for _, id := range ids {
		if roles, ok := m.byUserID[id]; ok {
			out[id] = roles
		}
	}
	return out, nil
}

// newAdminUserSrv builds a gqlgen handler.Server backed by a mock
// AdminUserUsecase. Other usecase fields are nil — only admin-user resolvers
// are exercised here.
func newAdminUserSrv(adminUC usecase.AdminUserUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, adminUC)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ctxWithRoles returns a context enriched with a loader.Loaders that has
// RoleByUserID backed by rolesByUser. Other loaders are nil and must not be
// invoked in the tests that use this helper.
func ctxWithRoles(base context.Context, rolesByUser map[string][]*domain.Role) context.Context {
	repo := &mockRoleByUserIDRepo{byUserID: rolesByUser}
	loaders := &loader.Loaders{
		RoleByUserID: dataloader.NewBatchedLoader(
			func(ctx context.Context, keys []string) []*dataloader.Result[[]*domain.Role] {
				out := make([]*dataloader.Result[[]*domain.Role], len(keys))
				result, err := repo.ListByUserIDs(ctx, keys)
				if err != nil {
					for i := range keys {
						out[i] = &dataloader.Result[[]*domain.Role]{Error: err}
					}
					return out
				}
				for i, k := range keys {
					roles := result[k]
					if roles == nil {
						roles = []*domain.Role{}
					}
					out[i] = &dataloader.Result[[]*domain.Role]{Data: roles}
				}
				return out
			},
		),
	}
	return loader.WithContext(base, loaders)
}

// ---------------------------------------------------------------------------
// Query.users tests
// ---------------------------------------------------------------------------

const usersQuery = `{"query":"{ users(first: 5) { edges { cursor node { id } } pageInfo { hasNextPage hasPreviousPage } totalCount } }"}`

// TestAdminUserResolver_Users_NonAdmin verifies that a non-admin caller
// (the mock usecase returns FORBIDDEN) gets errors[0].extensions.code ==
// "FORBIDDEN" from the users query.
func TestAdminUserResolver_Users_NonAdmin(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		listErr: gqlerr.NewForbidden("admin only"),
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("non-admin"), usersQuery)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q", code)
	}
}

// TestAdminUserResolver_Users_AdminHappyPath verifies that an admin caller
// gets a UserConnection with populated edges and totalCount.
func TestAdminUserResolver_Users_AdminHappyPath(t *testing.T) {
	t.Parallel()

	dn1, dn2 := "Alice", "Bob"
	mock := &mockAdminUserUsecase{
		listResult: &usecase.AdminUserConnection{
			Edges: []usecase.AdminUserEdge{
				{Cursor: "u1", Node: &domain.User{ID: "u1", DisplayName: &dn1}},
				{Cursor: "u2", Node: &domain.User{ID: "u2", DisplayName: &dn2}},
			},
			PageInfo:   usecase.PageInfo{HasNextPage: false, HasPreviousPage: false},
			TotalCount: 2,
		},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), usersQuery)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	conn, _ := data["users"].(map[string]any)
	if conn == nil {
		t.Fatalf("expected data.users, got nil; response: %v", resp)
	}
	edges, _ := conn["edges"].([]any)
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
	total, _ := conn["totalCount"].(float64)
	if int(total) != 2 {
		t.Fatalf("expected totalCount=2, got %v", conn["totalCount"])
	}
}

// ---------------------------------------------------------------------------
// Mutation.assignRole tests
// ---------------------------------------------------------------------------

const assignRoleMutation = `{"query":"mutation { assignRole(userId: \"u1\", roleId: \"r1\") { id } }"}`

// TestAdminUserResolver_AssignRole_HappyPath verifies that assignRole returns
// the updated user's ID when the usecase succeeds.
func TestAdminUserResolver_AssignRole_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		assignResult: &domain.User{ID: "u1"},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), assignRoleMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	user, _ := data["assignRole"].(map[string]any)
	if user == nil {
		t.Fatalf("expected data.assignRole, got nil; response: %v", resp)
	}
	if user["id"] != "u1" {
		t.Fatalf("expected id=u1, got %v", user["id"])
	}
}

// TestAdminUserResolver_AssignRole_Forbidden verifies that FORBIDDEN from the
// usecase propagates unchanged to the caller.
func TestAdminUserResolver_AssignRole_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		assignErr: gqlerr.NewForbidden("admin only"),
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("non-admin"), assignRoleMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q", code)
	}
}

// ---------------------------------------------------------------------------
// User.roles DataLoader test
// ---------------------------------------------------------------------------

// usersWithRolesQuery requests the users list and resolves roles for each
// user node.
const usersWithRolesQuery = `{"query":"{ users(first: 3) { edges { node { id roles { id name } } } } }"}`

// TestAdminUserResolver_Roles_ViaDataLoader verifies that User.roles is
// resolved through the RoleByUserID DataLoader (no N+1) and that the roles
// shape is correct. The test injects a loader context carrying a fake
// RoleByUserID DataLoader backed by an in-memory map.
func TestAdminUserResolver_Roles_ViaDataLoader(t *testing.T) {
	t.Parallel()

	adminRoleName := "admin"
	generalRoleName := "general"
	adminRole := &domain.Role{ID: "role-admin", Name: adminRoleName}
	generalRole := &domain.Role{ID: "role-general", Name: generalRoleName}

	mock := &mockAdminUserUsecase{
		listResult: &usecase.AdminUserConnection{
			Edges: []usecase.AdminUserEdge{
				{Cursor: "u1", Node: &domain.User{ID: "u1"}},
				{Cursor: "u2", Node: &domain.User{ID: "u2"}},
				{Cursor: "u3", Node: &domain.User{ID: "u3"}},
			},
			PageInfo:   usecase.PageInfo{},
			TotalCount: 3,
		},
	}
	srv := newAdminUserSrv(mock)

	rolesByUser := map[string][]*domain.Role{
		"u1": {adminRole, generalRole},
		"u2": {generalRole},
		"u3": {},
	}
	ctx := ctxWithRoles(authedCtx("admin"), rolesByUser)
	resp := gqlRequest(t, srv, ctx, usersWithRolesQuery)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	conn, _ := data["users"].(map[string]any)
	if conn == nil {
		t.Fatalf("expected data.users, got nil; response: %v", resp)
	}
	edges, _ := conn["edges"].([]any)
	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(edges))
	}

	// u1 should have 2 roles.
	u1, _ := edges[0].(map[string]any)["node"].(map[string]any)
	u1roles, _ := u1["roles"].([]any)
	if len(u1roles) != 2 {
		t.Fatalf("expected u1 to have 2 roles, got %d", len(u1roles))
	}

	// u2 should have 1 role.
	u2, _ := edges[1].(map[string]any)["node"].(map[string]any)
	u2roles, _ := u2["roles"].([]any)
	if len(u2roles) != 1 {
		t.Fatalf("expected u2 to have 1 role, got %d", len(u2roles))
	}

	// u3 should have 0 roles.
	u3, _ := edges[2].(map[string]any)["node"].(map[string]any)
	u3roles, _ := u3["roles"].([]any)
	if len(u3roles) != 0 {
		t.Fatalf("expected u3 to have 0 roles, got %d", len(u3roles))
	}
}

// ---------------------------------------------------------------------------
// Query.roles tests
// ---------------------------------------------------------------------------

const rolesQuery = `{"query":"{ roles { id name } }"}`

// TestAdminUserResolver_Roles_HappyPath verifies that Query.roles returns the
// expected role list when the usecase succeeds.
func TestAdminUserResolver_Roles_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		listRolesResult: []*domain.Role{
			{ID: "r-admin", Name: "admin"},
			{ID: "r-general", Name: "general"},
		},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), rolesQuery)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	roles, _ := data["roles"].([]any)
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d; response: %v", len(roles), resp)
	}
	first, _ := roles[0].(map[string]any)
	if first["id"] != "r-admin" || first["name"] != "admin" {
		t.Fatalf("roles[0] = %v, want {id:r-admin name:admin}", first)
	}
	second, _ := roles[1].(map[string]any)
	if second["id"] != "r-general" || second["name"] != "general" {
		t.Fatalf("roles[1] = %v, want {id:r-general name:general}", second)
	}
}

// TestAdminUserResolver_Roles_Forbidden verifies that FORBIDDEN from the
// usecase propagates unchanged.
func TestAdminUserResolver_Roles_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		listRolesErr: gqlerr.NewForbidden("admin only"),
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("non-admin"), rolesQuery)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q", code)
	}
}
