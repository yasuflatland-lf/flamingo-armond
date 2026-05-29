package resolver_test

import (
	"context"
	"sync"
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
	"backend/internal/usecase/ucerr"
)

// ---------------------------------------------------------------------------
// mockAdminUserUsecase — stub for AdminUserUsecase
// ---------------------------------------------------------------------------

type mockAdminUserUsecase struct {
	listResult    *usecase.AdminUserConnection
	listErr       error
	getResult     *domain.User
	getErr        error
	editOutcome   usecase.AdminEditUserOutcome
	editErr       error
	lastEditID    string
	lastEditInput usecase.AdminEditUserInput
}

func (m *mockAdminUserUsecase) List(_ context.Context, _, _ *int, _, _, _ *string) (*usecase.AdminUserConnection, error) {
	return m.listResult, m.listErr
}
func (m *mockAdminUserUsecase) Get(_ context.Context, _ string) (*domain.User, error) {
	return m.getResult, m.getErr
}

func (m *mockAdminUserUsecase) EditUser(_ context.Context, id string, input usecase.AdminEditUserInput) (usecase.AdminEditUserOutcome, error) {
	m.lastEditID = id
	m.lastEditInput = input
	return m.editOutcome, m.editErr
}

// mockRoleByUserIDRepo satisfies the minimal interface needed to build the
// RoleByUserID DataLoader.
type mockRoleByUserIDRepo struct {
	mu sync.Mutex
	// byUserID maps user id → roles returned for that user.
	byUserID      map[string][]*domain.Role
	listCallCount int
	// lastIDs holds the key slice from the most recent ListByUserIDs call so
	// N+1 batch assertions can verify all expected user IDs were batched together.
	lastIDs []string
}

func (m *mockRoleByUserIDRepo) ListByUserIDs(_ context.Context, ids []string) (map[string][]*domain.Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

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

func (m *mockRoleByUserIDRepo) ListByUser(_ context.Context, userID string) ([]*domain.Role, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listCallCount++
	m.lastIDs = []string{userID}
	roles := m.byUserID[userID]
	if roles == nil {
		return []*domain.Role{}, nil
	}
	return roles, nil
}

// newAdminUserSrv builds a gqlgen handler.Server backed by a mock
// AdminUserUsecase. Other usecase fields are nil — only admin-user resolvers
// are exercised here.
func newAdminUserSrv(adminUC usecase.AdminUserUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, adminUC, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// newAdminUserSrvWithAuth builds a server like newAdminUserSrv but also wires
// an AuthSvc backed by the given UserRoleRepository. Tests that exercise the
// User.roles admin gate need this because the resolver consults AuthSvc when
// the caller is reading another user's roles.
func newAdminUserSrvWithAuth(adminUC usecase.AdminUserUsecase, isAdmin bool, rolesByUser ...map[string][]*domain.Role) *handler.Server {
	roleRepo := &mockUserRoleRepository{isAdmin: isAdmin}
	authSvc := auth.NewService(roleRepo)
	userRoles := map[string][]*domain.Role{}
	if len(rolesByUser) > 0 && rolesByUser[0] != nil {
		userRoles = rolesByUser[0]
	}
	userUC := usecase.NewUserUsecase(&mockUserRepository{}, &mockRoleByUserIDRepo{byUserID: userRoles}, authSvc, newDiscardLogger())
	r := resolver.NewResolver(userUC, nil, nil, nil, authSvc, nil, adminUC, nil, nil, nil, nil)
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
		listErr: &ucerr.ForbiddenError{Message: "admin only"},
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

	mock := &mockAdminUserUsecase{
		listResult: &usecase.AdminUserConnection{
			Edges: []usecase.AdminUserEdge{
				{Cursor: "u1", Node: &domain.User{ID: "u1", DisplayName: dnPtr("Alice")}},
				{Cursor: "u2", Node: &domain.User{ID: "u2", DisplayName: dnPtr("Bob")}},
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
// User.roles DataLoader test
// ---------------------------------------------------------------------------

// usersWithRolesQuery requests the users list and resolves roles for each
// user node.
const usersWithRolesQuery = `{"query":"{ users(first: 3) { edges { node { id roles { id name } } } } }"}`

// TestAdminUserResolver_Roles_FromUsecase verifies that User.roles delegates
// to UserUsecase.RolesFor and returns the expected role shape.
func TestAdminUserResolver_Roles_FromUsecase(t *testing.T) {
	t.Parallel()

	adminRoleName := domain.RoleName("admin")
	generalRoleName := domain.RoleName("general")
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
	rolesByUser := map[string][]*domain.Role{
		"u1": {adminRole, generalRole},
		"u2": {generalRole},
		"u3": {},
	}
	// Caller is admin (isAdmin=true) so the User.roles gate lets the usecase
	// load roles for all three foreign user IDs.
	srv := newAdminUserSrvWithAuth(mock, true, rolesByUser)
	ctx := authedCtx("admin")
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

	ctx := authedCtx("u-caller")
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
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-self", DisplayName: dnPtr("Alice")},
	}
	rolesByUser := map[string][]*domain.Role{
		"u-self": {{ID: "r-general", Name: "general"}},
	}
	rolesRepo := &mockRoleByUserIDRepo{byUserID: rolesByUser}
	// isAdmin=false models a non-admin caller; the self-introspection branch
	// must skip the IsAdmin check entirely.
	roleRepo := &mockUserRoleRepository{isAdmin: false}
	authSvc := auth.NewService(roleRepo)
	uc := usecase.NewUserUsecase(userMock, rolesRepo, authSvc, newDiscardLogger())
	r := resolver.NewResolver(uc, nil, nil, nil, authSvc, nil, &mockAdminUserUsecase{}, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})

	ctx := authedCtx("u-self")
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
// Query.adminUser tests (I9)
// ---------------------------------------------------------------------------

// TestAdminUserResolver_AdminUser_HappyPath verifies that Query.adminUser
// returns the user with id and displayName populated when the user exists.
func TestAdminUserResolver_AdminUser_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		getResult: &domain.User{ID: "u-existing", DisplayName: dnPtr("Charlie")},
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
		getErr: &ucerr.ForbiddenError{Message: "admin only"},
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
// Mutation.adminEditUser tests
// ---------------------------------------------------------------------------

const adminEditUserMutation = `{"query":"mutation { adminEditUser(id: \"u-target\", input: { displayName: \"Dana\", bio: \"A short bio.\", roleIds: [\"r-admin\", \"r-general\"], expectedVersion: 41 }) { __typename ... on AdminEditUserSuccess { user { id displayName bio version roles { id name } } } ... on InputValidationError { field message } ... on CannotRevokeOwnAdminRoleError { message } ... on ConcurrentUpdateError { message } } }"}`

const adminEditUserMutationWithExpectedVersion = `{"query":"mutation { adminEditUser(id: \"u-target\", input: { displayName: \"Dana\", bio: \"A short bio.\", roleIds: [\"r-admin\", \"r-general\"], expectedVersion: 41 }) { __typename ... on AdminEditUserSuccess { user { id displayName bio version roles { id name } } } ... on InputValidationError { field message } ... on CannotRevokeOwnAdminRoleError { message } ... on ConcurrentUpdateError { message } } }"}`

const adminEditUserConcurrentMutation = `{"query":"mutation { adminEditUser(id: \"u-target\", input: { displayName: \"Dana\", roleIds: [], expectedVersion: 41 }) { __typename ... on AdminEditUserSuccess { user { id version } } ... on InputValidationError { field message } ... on CannotRevokeOwnAdminRoleError { message } ... on ConcurrentUpdateError { message } } }"}`

const adminEditUserForbiddenMutation = `{"query":"mutation { adminEditUser(id: \"u-target\", input: { roleIds: [], expectedVersion: 41 }) { __typename ... on AdminEditUserSuccess { user { id } } ... on InputValidationError { field message } ... on CannotRevokeOwnAdminRoleError { message } ... on ConcurrentUpdateError { message } } }"}`

func TestAdminUserResolver_AdminEditUser_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		editOutcome: usecase.AdminEditUserOutcome{
			User: &domain.User{
				ID:          "u-target",
				DisplayName: dnPtr("Dana"),
				Bio:         domain.BioFromPtr(ptr("A short bio.")),
			},
		},
	}
	rolesByUser := map[string][]*domain.Role{
		"u-target": {
			{ID: "r-admin", Name: "admin"},
			{ID: "r-general", Name: "general"},
		},
	}
	srv := newAdminUserSrvWithAuth(mock, true, rolesByUser)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminEditUserMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["adminEditUser"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.adminEditUser, got nil; response: %v", resp)
	}
	if payload["__typename"] != "AdminEditUserSuccess" {
		t.Fatalf("expected __typename=AdminEditUserSuccess, got %v", payload["__typename"])
	}
	user, _ := payload["user"].(map[string]any)
	if user == nil || user["id"] != "u-target" {
		t.Fatalf("expected user.id=u-target, got %v", payload["user"])
	}
	if user["displayName"] != "Dana" {
		t.Fatalf("expected displayName=Dana, got %v", user["displayName"])
	}
	roles, _ := user["roles"].([]any)
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
}

func TestAdminUserResolver_AdminEditUser_MapsExpectedVersionAndUserVersion(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		editOutcome: usecase.AdminEditUserOutcome{
			User: &domain.User{
				ID:          "u-target",
				DisplayName: dnPtr("Dana"),
				Bio:         domain.BioFromPtr(ptr("A short bio.")),
				Version:     41,
			},
		},
	}
	rolesByUser := map[string][]*domain.Role{
		"u-target": {
			{ID: "r-admin", Name: "admin"},
			{ID: "r-general", Name: "general"},
		},
	}
	srv := newAdminUserSrvWithAuth(mock, true, rolesByUser)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminEditUserMutationWithExpectedVersion)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	if mock.lastEditID != "u-target" {
		t.Fatalf("expected EditUser id=u-target, got %q", mock.lastEditID)
	}
	if mock.lastEditInput.ExpectedVersion != 41 {
		t.Fatalf("expected ExpectedVersion=41, got %d", mock.lastEditInput.ExpectedVersion)
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["adminEditUser"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.adminEditUser, got nil; response: %v", resp)
	}
	user, _ := payload["user"].(map[string]any)
	if user == nil {
		t.Fatalf("expected payload.user, got nil; response: %v", resp)
	}
	if version, _ := user["version"].(float64); int(version) != 41 {
		t.Fatalf("expected version=41, got %v", user["version"])
	}
}

func TestAdminUserResolver_AdminEditUser_ConcurrentUpdate(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		editOutcome: usecase.AdminEditUserOutcome{ConcurrentUpdate: true},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminEditUserConcurrentMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["adminEditUser"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.adminEditUser, got nil; response: %v", resp)
	}
	if payload["__typename"] != "ConcurrentUpdateError" {
		t.Fatalf("expected __typename=ConcurrentUpdateError, got %v", payload["__typename"])
	}
	if payload["message"] != "This user was changed by someone else. Reload and try again." {
		t.Fatalf("expected concurrent update message, got %v", payload["message"])
	}
}

func TestAdminUserResolver_AdminEditUser_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		editErr: &ucerr.ForbiddenError{Message: "admin only"},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("non-admin"), adminEditUserForbiddenMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}
}

func TestAdminUserResolver_AdminEditUser_InputValidation(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		editOutcome: usecase.AdminEditUserOutcome{
			Validation: &usecase.InputValidationInfo{
				Field:   "roleIds",
				Message: "role not found",
			},
		},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminEditUserMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["adminEditUser"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.adminEditUser, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v", payload["__typename"])
	}
	if payload["field"] != "roleIds" {
		t.Fatalf("expected field=roleIds, got %v", payload["field"])
	}
	if payload["message"] != "role not found" {
		t.Fatalf("expected message='role not found', got %v", payload["message"])
	}
}

func TestAdminUserResolver_AdminEditUser_CannotRevokeOwnAdmin(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		editOutcome: usecase.AdminEditUserOutcome{CannotRevokeOwnAdmin: true},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminEditUserMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["adminEditUser"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.adminEditUser, got nil; response: %v", resp)
	}
	if payload["__typename"] != "CannotRevokeOwnAdminRoleError" {
		t.Fatalf("expected __typename=CannotRevokeOwnAdminRoleError, got %v", payload["__typename"])
	}
	if payload["message"] != "Cannot revoke your own admin role" {
		t.Fatalf("expected self-demotion message, got %v", payload["message"])
	}
}

func TestAdminUserResolver_AdminEditUser_XORInvariantViolation(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		editOutcome: usecase.AdminEditUserOutcome{},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminEditUserMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL_SERVER_ERROR, got %q; response: %v", code, resp)
	}
}
