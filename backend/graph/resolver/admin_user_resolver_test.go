package resolver_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/graph-gophers/dataloader/v7"

	"backend/graph/generated"
	"backend/graph/model"
	"backend/graph/resolver"
	"backend/internal/cursor"
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

	deleteErr    error
	lastDeleteID string
	deleteCalls  int
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

func (m *mockAdminUserUsecase) DeleteUser(_ context.Context, id string) error {
	m.deleteCalls++
	m.lastDeleteID = id
	return m.deleteErr
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
	// adminCount is returned by CountAdmins; used by DeleteMyAccount tests.
	adminCount int64
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

func (m *mockRoleByUserIDRepo) CountAdmins(_ context.Context) (int64, error) {
	return m.adminCount, nil
}

// newAdminUserSrv builds a gqlgen handler.Server backed by a mock
// AdminUserUsecase. Other usecase fields are nil — only admin-user resolvers
// are exercised here.
func newAdminUserSrv(adminUC usecase.AdminUserUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, adminUC, nil, nil, nil, nil, nil, nil, nil, nil)
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

// ctxWithRolesLoaderError installs a RoleByUserID loader whose batch function
// fails every key with loadErr. Used to exercise the resolver's loader-error
// classification (CANCELLED for context errors, INTERNAL otherwise).
func ctxWithRolesLoaderError(base context.Context, loadErr error) context.Context {
	loaders := &loader.Loaders{
		RoleByUserID: dataloader.NewBatchedLoader(
			func(_ context.Context, keys []string) []*dataloader.Result[[]*domain.Role] {
				out := make([]*dataloader.Result[[]*domain.Role], len(keys))
				for i := range keys {
					out[i] = &dataloader.Result[[]*domain.Role]{Error: loadErr}
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

const usersQuery = `{"query":"{ users(first: 5) { edges { cursor node { id } } pageInfo { hasNextPage hasPreviousPage startCursor endCursor } totalCount } }"}`

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
			Users: []*domain.User{
				{ID: "u1", DisplayName: dnPtr("Alice")},
				{ID: "u2", DisplayName: dnPtr("Bob")},
			},
			StartCur:   "u1",
			EndCur:     "u2",
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

	// The admin-user connection now emits opaque v1 cursors via buildEdges,
	// exactly like the other four connections — never the raw user UUID. Each
	// edge cursor is cursor.Encode(node.id) and round-trips back to the raw id.
	for i, want := range []string{"u1", "u2"} {
		edge, _ := edges[i].(map[string]any)
		gotCur, _ := edge["cursor"].(string)
		if gotCur != cursor.Encode(want) {
			t.Fatalf("edges[%d].cursor = %q, want encoded %q", i, gotCur, cursor.Encode(want))
		}
		if gotCur == want {
			t.Fatalf("edges[%d].cursor = %q must not be the raw user id", i, gotCur)
		}
		dec, err := cursor.Decode(gotCur)
		if err != nil || dec != want {
			t.Fatalf("edges[%d].cursor decode = (%q, %v), want (%q, nil)", i, dec, err, want)
		}
	}

	// PageInfo start/end cursors are encoded once at the resolver boundary from
	// the usecase's RAW StartCur/EndCur.
	pageInfo, _ := conn["pageInfo"].(map[string]any)
	if got, _ := pageInfo["startCursor"].(string); got != cursor.Encode("u1") {
		t.Fatalf("pageInfo.startCursor = %q, want %q", got, cursor.Encode("u1"))
	}
	if got, _ := pageInfo["endCursor"].(string); got != cursor.Encode("u2") {
		t.Fatalf("pageInfo.endCursor = %q, want %q", got, cursor.Encode("u2"))
	}
}

// ---------------------------------------------------------------------------
// User.roles DataLoader test
// ---------------------------------------------------------------------------

// usersWithRolesQuery requests the users list and resolves roles for each
// user node.
const usersWithRolesQuery = `{"query":"{ users(first: 3) { edges { node { id roles { id name } } } } }"}`

// TestAdminUserResolver_Roles_BatchesIntoOneQueryPerPage verifies that
// resolving User.roles for every node in the admin users list issues a single
// batched roles query for the whole page — the caller's own admin-status key
// joins the same batch — instead of one query per row (the N+1 this change
// removes). It also pins the role shape returned for each user.
func TestAdminUserResolver_Roles_BatchesIntoOneQueryPerPage(t *testing.T) {
	t.Parallel()

	adminRole := &domain.Role{ID: "role-admin", Name: domain.AdminRoleName}
	generalRole := &domain.Role{ID: "role-general", Name: domain.GeneralRoleName}

	mock := &mockAdminUserUsecase{
		listResult: &usecase.AdminUserConnection{
			Users: []*domain.User{
				{ID: "u1"},
				{ID: "u2"},
				{ID: "u3"},
			},
			TotalCount: 3,
		},
	}
	// The caller "admin" holds the admin role so the gate passes for the three
	// foreign rows; its key rides the same request batch as u1/u2/u3.
	repo := &mockRoleByUserIDRepo{byUserID: map[string][]*domain.Role{
		"admin": {adminRole},
		"u1":    {adminRole, generalRole},
		"u2":    {generalRole},
		"u3":    {},
	}}
	srv := newAdminUserSrv(mock)
	ctx := ctxWithRolesRepo(authedCtx("admin"), repo)
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

	// u1 → 2 roles, u2 → 1 role, u3 → 0 roles.
	for i, want := range []int{2, 1, 0} {
		node, _ := edges[i].(map[string]any)["node"].(map[string]any)
		roles, _ := node["roles"].([]any)
		if len(roles) != want {
			t.Fatalf("edges[%d] roles = %d, want %d", i, len(roles), want)
		}
	}

	// The N+1 kill: exactly one batched ListByUserIDs call for the whole page,
	// not one per row. mockRoleByUserIDRepo.ListByUser also increments
	// listCallCount, so this simultaneously proves the non-batched path is
	// never taken.
	if got := repo.listCallCount; got != 1 {
		t.Fatalf("expected exactly 1 batched roles query per page, got %d (keys: %v)", got, repo.lastIDs)
	}
	// The caller's admin-status key joined the page batch (4 unique keys:
	// caller + 3 rows), confirming no separate round trip for the admin check.
	if len(repo.lastIDs) != 4 {
		t.Fatalf("expected the single batch to carry caller + 3 rows, got keys: %v", repo.lastIDs)
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
	srv := newAdminUserSrv(mock)

	// The caller holds only a non-admin role, so the gate rejects reading a
	// foreign user's roles.
	generalRole := &domain.Role{ID: "role-general", Name: domain.GeneralRoleName}
	ctx := ctxWithRoles(authedCtx("u-caller"), map[string][]*domain.Role{
		"u-caller": {generalRole},
	})
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

	// me { ... } resolves via UserUsecase; the roles field routes through the
	// per-request loader. A non-admin caller may still read their own roles
	// because caller.Sub == obj.ID skips the admin gate entirely.
	userMock := &mockUserRepository{
		findResult: &domain.User{ID: "u-self", DisplayName: dnPtr("Alice")},
	}
	srv := newServer(userMock)

	ctx := ctxWithRoles(authedCtx("u-self"), map[string][]*domain.Role{
		"u-self": {{ID: "r-general", Name: domain.GeneralRoleName}},
	})
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
// User.roles resolver — direct-unit error/guard branches
// ---------------------------------------------------------------------------

// TestUserResolver_Roles_Unauthenticated verifies that an anonymous caller
// (no auth user on the context) receives UNAUTHENTICATED even when the loader
// is installed.
func TestUserResolver_Roles_Unauthenticated(t *testing.T) {
	t.Parallel()

	ctx := ctxWithRoles(context.Background(), map[string][]*domain.Role{"u-1": {}})
	_, err := (&resolver.Resolver{}).User().Roles(ctx, &model.User{ID: "u-1"})
	if !gqlerr.IsCode(err, gqlerr.CodeUnauthenticated) {
		t.Fatalf("want UNAUTHENTICATED, got %v", err)
	}
}

// TestUserResolver_Roles_MissingLoaderInternal verifies that resolving roles
// without the DataLoader middleware installed returns INTERNAL.
func TestUserResolver_Roles_MissingLoaderInternal(t *testing.T) {
	t.Parallel()

	_, err := (&resolver.Resolver{}).User().Roles(authedCtx("u-1"), &model.User{ID: "u-1"})
	if !gqlerr.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL when loader middleware is not installed, got %v", err)
	}
}

// TestUserResolver_Roles_LoaderErrorInternal verifies that a generic loader
// failure on the self path (caller.Sub == obj.ID) classifies as INTERNAL.
func TestUserResolver_Roles_LoaderErrorInternal(t *testing.T) {
	t.Parallel()

	ctx := ctxWithRolesLoaderError(authedCtx("u-1"), errors.New("db down"))
	_, err := (&resolver.Resolver{}).User().Roles(ctx, &model.User{ID: "u-1"})
	if !gqlerr.IsCode(err, gqlerr.CodeInternal) {
		t.Fatalf("want INTERNAL for a generic loader error, got %v", err)
	}
}

// TestUserResolver_Roles_ContextCancelled verifies that a cancelled-context
// loader error classifies as CANCELLED.
func TestUserResolver_Roles_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx := ctxWithRolesLoaderError(authedCtx("u-1"), context.Canceled)
	_, err := (&resolver.Resolver{}).User().Roles(ctx, &model.User{ID: "u-1"})
	if !gqlerr.IsCode(err, gqlerr.CodeCancelled) {
		t.Fatalf("want CANCELLED for a cancelled-context loader error, got %v", err)
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
		"admin": {{ID: "r-admin", Name: domain.AdminRoleName}},
		"u-target": {
			{ID: "r-admin", Name: "admin"},
			{ID: "r-general", Name: "general"},
		},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, ctxWithRoles(authedCtx("admin"), rolesByUser), adminEditUserMutation)

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
		"admin": {{ID: "r-admin", Name: domain.AdminRoleName}},
		"u-target": {
			{ID: "r-admin", Name: "admin"},
			{ID: "r-general", Name: "general"},
		},
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, ctxWithRoles(authedCtx("admin"), rolesByUser), adminEditUserMutationWithExpectedVersion)

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

// ---------------------------------------------------------------------------
// Mutation.adminDeleteUser tests
// ---------------------------------------------------------------------------

const adminDeleteUserMutation = `{"query":"mutation { adminDeleteUser(id: \"u-target\") }"}`

// TestAdminUserResolver_AdminDeleteUser_Success verifies adminDeleteUser returns
// true and delegates to the usecase with the requested id when the usecase
// succeeds.
func TestAdminUserResolver_AdminDeleteUser_Success(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminDeleteUserMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	if deleted, _ := data["adminDeleteUser"].(bool); !deleted {
		t.Fatalf("expected data.adminDeleteUser == true, got %v", data["adminDeleteUser"])
	}
	if mock.deleteCalls != 1 || mock.lastDeleteID != "u-target" {
		t.Fatalf("usecase.DeleteUser: calls=%d id=%q, want 1 and \"u-target\"", mock.deleteCalls, mock.lastDeleteID)
	}
}

// TestAdminUserResolver_AdminDeleteUser_Forbidden verifies a usecase forbidden
// error (self-deletion or last-admin guard) surfaces as FORBIDDEN.
func TestAdminUserResolver_AdminDeleteUser_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		deleteErr: ucerr.NewForbiddenError("cannot delete the last admin account"),
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminDeleteUserMutation)

	if code := errCode(t, resp); code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}
}

// TestAdminUserResolver_AdminDeleteUser_NotFound verifies a usecase validation
// error (missing target) surfaces as BAD_USER_INPUT.
func TestAdminUserResolver_AdminDeleteUser_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockAdminUserUsecase{
		deleteErr: ucerr.NewValidationError("id", "user not found"),
	}
	srv := newAdminUserSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), adminDeleteUserMutation)

	if code := errCode(t, resp); code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected BAD_USER_INPUT, got %q; response: %v", code, resp)
	}
}
