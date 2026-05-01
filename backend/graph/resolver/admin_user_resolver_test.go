package resolver_test

import (
	"context"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/generated"
	"backend/graph/resolver"
	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/loader"
	"backend/internal/usecase"
)

// ---------------------------------------------------------------------------
// mockAdminUserUsecase — stub for AdminUserUsecase
// ---------------------------------------------------------------------------

type mockAdminUserUsecase struct {
	listResult   *usecase.AdminUserConnection
	listErr      error
	getResult    *domain.User
	getErr       error
	updateResult *domain.User
	updateErr    error
	assignResult *domain.User
	assignErr    error
	revokeResult *domain.User
	revokeErr    error
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

// mockRoleByUserIDRepo satisfies the minimal interface needed to build the
// RoleByUserID DataLoader.
type mockRoleByUserIDRepo struct {
	// byUserID maps user id → roles returned for that user.
	byUserID      map[string][]*domain.Role
	listCallCount int
	// lastIDs holds the key slice from the most recent ListByUserIDs call so
	// N+1 batch assertions can verify all expected user IDs were batched together.
	lastIDs []string
}

func (m *mockRoleByUserIDRepo) ListByUserIDs(_ context.Context, ids []string) (map[string][]*domain.Role, error) {
	m.listCallCount++
	m.lastIDs = append([]string(nil), ids...)
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
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, adminUC, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// newAdminUserSrvWithAuth builds a server like newAdminUserSrv but also wires
// an AuthSvc backed by the given UserRoleRepository. Tests that exercise the
// User.roles admin gate need this because the resolver consults AuthSvc when
// the caller is reading another user's roles.
func newAdminUserSrvWithAuth(adminUC usecase.AdminUserUsecase, isAdmin bool) *handler.Server {
	roleRepo := &mockUserRoleRepository{isAdmin: isAdmin}
	r := resolver.NewResolver(nil, nil, nil, nil, auth.NewService(roleRepo), nil, adminUC, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ctxWithRoles returns a context enriched with a loader.Loaders that has
// RoleByUserID backed by rolesByUser. Other loaders are nil and must not be
// invoked in the tests that use this helper.
func ctxWithRoles(base context.Context, rolesByUser map[string][]*domain.Role) context.Context {
	repo := &mockRoleByUserIDRepo{byUserID: rolesByUser}
	return ctxWithRolesRepo(base, repo)
}

// ctxWithRolesRepo is like ctxWithRoles but accepts a pre-constructed
// mockRoleByUserIDRepo so callers can inspect its listCallCount after the
// GraphQL request resolves.
func ctxWithRolesRepo(base context.Context, repo *mockRoleByUserIDRepo) context.Context {
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
//
// N+1 assertion (C3): three users are fetched; all three user IDs must reach
// the batch function in exactly one call — the DataLoader must coalesce the
// individual Load calls into a single ListByUserIDs invocation.
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
	// Caller is admin (isAdmin=true) so the User.roles gate lets the
	// DataLoader fan out to all three foreign user IDs.
	srv := newAdminUserSrvWithAuth(mock, true)

	rolesByUser := map[string][]*domain.Role{
		"u1": {adminRole, generalRole},
		"u2": {generalRole},
		"u3": {},
	}
	// Use the repo-exposing variant so we can assert on the batch call count.
	roleRepo := &mockRoleByUserIDRepo{byUserID: rolesByUser}
	ctx := ctxWithRolesRepo(authedCtx("admin"), roleRepo)
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

	// N+1 assertion: all three user IDs must have been batched into exactly
	// one ListByUserIDs call. More than one call indicates the DataLoader did
	// not coalesce the individual Load() calls, i.e. an N+1 query per user.
	if roleRepo.listCallCount != 1 {
		t.Fatalf("expected exactly 1 batched DB call, got %d (N+1 regression)", roleRepo.listCallCount)
	}
	wantIDs := []string{"u1", "u2", "u3"}
	for _, id := range wantIDs {
		found := false
		for _, got := range roleRepo.lastIDs {
			if got == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("batch call missing user ID %q; got %v", id, roleRepo.lastIDs)
		}
	}
	if len(roleRepo.lastIDs) != len(wantIDs) {
		t.Fatalf("batch call received %d user IDs, want %d; got %v", len(roleRepo.lastIDs), len(wantIDs), roleRepo.lastIDs)
	}
}

// ---------------------------------------------------------------------------
// User.roles admin gate
// ---------------------------------------------------------------------------

// TestAdminUserResolver_Roles_NonAdminNonSelf_Forbidden verifies that a
// non-admin caller querying another user's roles via adminUser.roles receives
// a FORBIDDEN error and that no roles are returned in the payload. Returning
// an empty slice silently would still leak the field's existence; the gate
// surfaces the rejection explicitly.
func TestAdminUserResolver_Roles_NonAdminNonSelf_Forbidden(t *testing.T) {
	t.Parallel()

	// adminUser query bypasses the AdminUserUsecase admin gate via the mock,
	// so the resolver-level gate on User.roles is what we exercise here. The
	// mock returns a target user whose ID differs from the caller's sub.
	mock := &mockAdminUserUsecase{
		getResult: &domain.User{ID: "u-target"},
	}
	// isAdmin=false models a non-admin caller.
	srv := newAdminUserSrvWithAuth(mock, false)

	rolesByUser := map[string][]*domain.Role{
		"u-target": {{ID: "r-admin", Name: "admin"}},
	}
	ctx := ctxWithRoles(authedCtx("u-caller"), rolesByUser)
	body := `{"query":"{ adminUser(id: \"u-target\") { id roles { id name } } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}

	// Payload must not surface a roles list — gqlgen sets data.adminUser to
	// null when a non-nullable field errors. Either branch is acceptable: the
	// FORBIDDEN code is what guards the leak.
	data, _ := resp["data"].(map[string]any)
	if data != nil {
		if user, ok := data["adminUser"].(map[string]any); ok {
			if _, hasRoles := user["roles"]; hasRoles {
				t.Fatalf("expected no roles field in payload on FORBIDDEN, got %v", user)
			}
		}
	}
}

// TestAdminUserResolver_Roles_SelfIntrospection_Allowed verifies that a
// non-admin caller may read their own roles via me { roles } without a
// FORBIDDEN. The User.roles resolver allows caller.Sub == obj.ID.
func TestAdminUserResolver_Roles_SelfIntrospection_Allowed(t *testing.T) {
	t.Parallel()

	// Wire UserUsecase so me { ... } can resolve. The AdminUserUsecase is
	// unused by this query; pass the mock to satisfy the resolver wiring.
	displayName := "Alice"
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-self", DisplayName: &displayName},
	}
	uc := usecase.NewUserUsecase(userMock)
	// isAdmin=false models a non-admin caller; the self-introspection branch
	// must skip the IsAdmin check entirely.
	roleRepo := &mockUserRoleRepository{isAdmin: false}
	r := resolver.NewResolver(uc, nil, nil, nil, auth.NewService(roleRepo), nil, &mockAdminUserUsecase{}, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	rolesByUser := map[string][]*domain.Role{
		"u-self": {{ID: "r-general", Name: "general"}},
	}
	ctx := ctxWithRoles(authedCtx("u-self"), rolesByUser)
	body := `{"query":"{ me { id roles { id name } } }"}`
	resp := gqlRequest(t, srv, ctx, body)

	if errs, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", errs)
	}
	data, _ := resp["data"].(map[string]any)
	me, _ := data["me"].(map[string]any)
	if me == nil {
		t.Fatalf("expected data.me, got nil; response: %v", resp)
	}
	roles, _ := me["roles"].([]any)
	if len(roles) != 1 {
		t.Fatalf("expected 1 role for self, got %d", len(roles))
	}
	first, _ := roles[0].(map[string]any)
	if first["id"] != "r-general" || first["name"] != "general" {
		t.Fatalf("roles[0] = %v, want {id:r-general name:general}", first)
	}
}

// ---------------------------------------------------------------------------
// Mutation.revokeRole tests (C6)
// ---------------------------------------------------------------------------

const revokeRoleMutation = `{"query":"mutation { revokeRole(userId: \"u1\", roleId: \"r1\") { id } }"}`
const revokeRoleSelfMutation = `{"query":"mutation { revokeRole(userId: \"admin\", roleId: \"r-admin\") { id } }"}`

// TestAdminUserResolver_RevokeRole_HappyPath verifies that an admin caller
// revoking a role from another user gets the updated user back with no error.
func TestAdminUserResolver_RevokeRole_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		revokeResult: &domain.User{ID: "u1"},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), revokeRoleMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	user, _ := data["revokeRole"].(map[string]any)
	if user == nil {
		t.Fatalf("expected data.revokeRole, got nil; response: %v", resp)
	}
	if user["id"] != "u1" {
		t.Fatalf("expected id=u1, got %v", user["id"])
	}
}

// TestAdminUserResolver_RevokeRole_SelfDemotionForbidden verifies that an
// admin caller attempting to revoke their own admin role receives FORBIDDEN.
// This covers the self-demotion guard in the usecase.
func TestAdminUserResolver_RevokeRole_SelfDemotionForbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		revokeErr: gqlerr.NewForbidden("cannot revoke own admin role"),
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), revokeRoleSelfMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}
}

// TestAdminUserResolver_RevokeRole_NonAdmin verifies that a non-admin caller
// receives FORBIDDEN when attempting to revoke a role.
func TestAdminUserResolver_RevokeRole_NonAdmin(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		revokeErr: gqlerr.NewForbidden("admin only"),
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("non-admin"), revokeRoleMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}
}

// ---------------------------------------------------------------------------
// Query.adminUser tests (I9)
// ---------------------------------------------------------------------------

// TestAdminUserResolver_AdminUser_HappyPath verifies that Query.adminUser
// returns the user with id and displayName populated when the user exists.
func TestAdminUserResolver_AdminUser_HappyPath(t *testing.T) {
	t.Parallel()

	displayName := "Charlie"
	mock := &mockAdminUserUsecase{
		getResult: &domain.User{ID: "u-existing", DisplayName: &displayName},
	}
	srv := newAdminUserSrv(mock)
	body := `{"query":"{ adminUser(id: \"u-existing\") { id displayName } }"}`
	resp := gqlRequest(t, srv, authedCtx("admin"), body)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	user, _ := data["adminUser"].(map[string]any)
	if user == nil {
		t.Fatalf("expected data.adminUser, got nil; response: %v", resp)
	}
	if user["id"] != "u-existing" {
		t.Fatalf("expected id=u-existing, got %v", user["id"])
	}
	if user["displayName"] != "Charlie" {
		t.Fatalf("expected displayName=Charlie, got %v", user["displayName"])
	}
}

// TestAdminUserResolver_AdminUser_NotFound_NullPayload verifies that
// Query.adminUser returns data.adminUser == null with NO errors when the
// user does not exist. This is the SSR redirect-to-list regression guard:
// the resolver returns (nil, nil) from the usecase so gqlgen renders the
// nullable field as null without setting errors[].
func TestAdminUserResolver_AdminUser_NotFound_NullPayload(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		// getResult == nil and getErr == nil: usecase found no user but
		// returned no error (missing row maps to (nil, nil)).
		getResult: nil,
		getErr:    nil,
	}
	srv := newAdminUserSrv(mock)
	body := `{"query":"{ adminUser(id: \"missing-id\") { id displayName } }"}`
	resp := gqlRequest(t, srv, authedCtx("admin"), body)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("expected no errors for missing user, got %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected data field, got nil; response: %v", resp)
	}
	// adminUser must be explicitly null, not missing.
	if _, exists := data["adminUser"]; !exists {
		t.Fatalf("expected data.adminUser key (null), got absent; response: %v", resp)
	}
	if data["adminUser"] != nil {
		t.Fatalf("expected data.adminUser == null, got %v", data["adminUser"])
	}
}

// TestAdminUserResolver_AdminUser_Forbidden verifies that a non-admin caller
// receives FORBIDDEN when querying adminUser.
func TestAdminUserResolver_AdminUser_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		getErr: gqlerr.NewForbidden("admin only"),
	}
	srv := newAdminUserSrv(mock)
	body := `{"query":"{ adminUser(id: \"u1\") { id } }"}`
	resp := gqlRequest(t, srv, authedCtx("non-admin"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}
}

// ---------------------------------------------------------------------------
// Mutation.adminUpdateUser tests (I9)
// ---------------------------------------------------------------------------

// TestAdminUserResolver_AdminUpdateUser_HappyPath verifies that an admin caller
// updating displayName/bio gets the updated user back in the payload.
func TestAdminUserResolver_AdminUpdateUser_HappyPath(t *testing.T) {
	t.Parallel()

	newName := "Dana"
	newBio := "A short bio."
	mock := &mockAdminUserUsecase{
		updateResult: &domain.User{ID: "u-target", DisplayName: &newName, Bio: &newBio},
	}
	srv := newAdminUserSrv(mock)
	body := `{"query":"mutation { adminUpdateUser(id: \"u-target\", input: { displayName: \"Dana\", bio: \"A short bio.\" }) { id displayName bio } }"}`
	resp := gqlRequest(t, srv, authedCtx("admin"), body)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	user, _ := data["adminUpdateUser"].(map[string]any)
	if user == nil {
		t.Fatalf("expected data.adminUpdateUser, got nil; response: %v", resp)
	}
	if user["id"] != "u-target" {
		t.Fatalf("expected id=u-target, got %v", user["id"])
	}
	if user["displayName"] != "Dana" {
		t.Fatalf("expected displayName=Dana, got %v", user["displayName"])
	}
	if user["bio"] != "A short bio." {
		t.Fatalf("expected bio=%q, got %v", "A short bio.", user["bio"])
	}
}

// TestAdminUserResolver_AdminUpdateUser_Forbidden verifies that a non-admin
// caller attempting adminUpdateUser receives FORBIDDEN.
func TestAdminUserResolver_AdminUpdateUser_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		updateErr: gqlerr.NewForbidden("admin only"),
	}
	srv := newAdminUserSrv(mock)
	body := `{"query":"mutation { adminUpdateUser(id: \"u1\", input: { displayName: \"Eve\" }) { id } }"}`
	resp := gqlRequest(t, srv, authedCtx("non-admin"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}
}
