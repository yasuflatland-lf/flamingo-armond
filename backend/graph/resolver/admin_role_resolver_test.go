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

// ---------------------------------------------------------------------------
// mockAdminRoleUsecase — stub for AdminRoleUsecase
// ---------------------------------------------------------------------------

type mockAdminRoleUsecase struct {
	listResult    []*domain.Role
	listErr       error
	getResult     *domain.Role
	getErr        error
	createOutcome usecase.CreateRoleOutcome
	createErr     error
	updateOutcome usecase.UpdateRoleOutcome
	updateErr     error
	deleteErr     error
}

func (m *mockAdminRoleUsecase) List(_ context.Context) ([]*domain.Role, error) {
	return m.listResult, m.listErr
}
func (m *mockAdminRoleUsecase) Get(_ context.Context, _ string) (*domain.Role, error) {
	return m.getResult, m.getErr
}
func (m *mockAdminRoleUsecase) Create(_ context.Context, _ string) (usecase.CreateRoleOutcome, error) {
	return m.createOutcome, m.createErr
}
func (m *mockAdminRoleUsecase) Update(_ context.Context, _, _ string) (usecase.UpdateRoleOutcome, error) {
	return m.updateOutcome, m.updateErr
}
func (m *mockAdminRoleUsecase) Delete(_ context.Context, _ string) error {
	return m.deleteErr
}

// newAdminRoleSrv builds a gqlgen handler.Server backed by a mock
// AdminRoleUsecase. Other usecase fields are nil — only admin-role resolvers
// are exercised here.
func newAdminRoleSrv(roleUC usecase.AdminRoleUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, roleUC, nil, nil, nil, nil)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ---------------------------------------------------------------------------
// Mutation.createRole tests
// ---------------------------------------------------------------------------

// createRoleMutation selects across both variants of the CreateRoleResult
// union so a single mutation body covers the success and input-validation
// cases.
const createRoleMutation = `{"query":"mutation { createRole(name: \"editor\") { __typename ... on CreateRoleSuccess { role { id name } } ... on InputValidationError { field message } } }"}`

// TestResolver_CreateRole_Success verifies that createRole returns the new
// role's id and name when the usecase succeeds, via the CreateRoleSuccess
// union variant.
func TestResolver_CreateRole_Success(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		createOutcome: usecase.CreateRoleOutcome{
			Role: &domain.Role{ID: "r-editor", Name: "editor"},
		},
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), createRoleMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createRole"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.createRole, got nil; response: %v", resp)
	}
	if payload["__typename"] != "CreateRoleSuccess" {
		t.Fatalf("expected __typename=CreateRoleSuccess, got %v", payload["__typename"])
	}
	role, _ := payload["role"].(map[string]any)
	if role == nil {
		t.Fatalf("expected role payload, got nil; response: %v", resp)
	}
	if role["id"] != "r-editor" {
		t.Fatalf("expected id=r-editor, got %v", role["id"])
	}
	if role["name"] != "editor" {
		t.Fatalf("expected name=editor, got %v", role["name"])
	}
}

// TestResolver_CreateRole_Forbidden verifies that FORBIDDEN from the usecase
// propagates unchanged to the caller.
func TestResolver_CreateRole_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		createErr: &ucerr.ForbiddenError{Message: "admin only"},
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("non-admin"), createRoleMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q", code)
	}
}

// TestResolver_CreateRole_InputValidation_DuplicateName verifies that an
// outcome carrying a Validation slot (e.g. duplicate role name) maps to the
// InputValidationError union variant — surfaced as data, not as an error.
func TestResolver_CreateRole_InputValidation_DuplicateName(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		createOutcome: usecase.CreateRoleOutcome{
			Validation: &usecase.InputValidationInfo{
				Field:   "name",
				Message: "role name already exists",
			},
		},
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), createRoleMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["createRole"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.createRole, got nil; response: %v", resp)
	}
	if payload["__typename"] != "InputValidationError" {
		t.Fatalf("expected __typename=InputValidationError, got %v", payload["__typename"])
	}
	if payload["field"] != "name" {
		t.Fatalf("expected field=name, got %v", payload["field"])
	}
	if payload["message"] != "role name already exists" {
		t.Fatalf("expected message='role name already exists', got %v", payload["message"])
	}
}

// TestResolver_CreateRole_XORInvariantViolation covers the defensive guard
// where the usecase returns a CreateRoleOutcome with no variant set. Must
// surface as INTERNAL — the resolver refuses to render an unselectable union
// value.
func TestResolver_CreateRole_XORInvariantViolation(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		createOutcome: usecase.CreateRoleOutcome{}, // no variant set
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), createRoleMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL_SERVER_ERROR, got %q; response: %v", code, resp)
	}
}

// ---------------------------------------------------------------------------
// Mutation.updateRole tests
// ---------------------------------------------------------------------------

// updateRoleMutation queries every variant of the UpdateRoleResult union so a
// single mutation body covers both the success and system-role-conflict cases.
const updateRoleMutation = `{"query":"mutation { updateRole(id: \"r1\", name: \"reviewer\") { __typename ... on UpdateRoleSuccess { role { id name } } ... on CannotModifySystemRoleError { message roleId roleName } } }"}`

// TestResolver_UpdateRole_Success verifies that the happy-path outcome maps to
// the UpdateRoleSuccess union variant carrying the renamed role.
func TestResolver_UpdateRole_Success(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		updateOutcome: usecase.UpdateRoleOutcome{
			Role: &domain.Role{ID: "r1", Name: "reviewer"},
		},
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), updateRoleMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateRole"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.updateRole, got nil; response: %v", resp)
	}
	if payload["__typename"] != "UpdateRoleSuccess" {
		t.Fatalf("expected __typename=UpdateRoleSuccess, got %v", payload["__typename"])
	}
	role, _ := payload["role"].(map[string]any)
	if role == nil || role["id"] != "r1" || role["name"] != "reviewer" {
		t.Fatalf("expected role={id:r1 name:reviewer}, got %v", role)
	}
}

// TestResolver_UpdateRole_SystemRoleConflict verifies that the system-role
// refusal outcome maps to the CannotModifySystemRoleError union variant
// (data, not an error) with the offending role's identity attached.
func TestResolver_UpdateRole_SystemRoleConflict(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		updateOutcome: usecase.UpdateRoleOutcome{
			SystemRoleConflict: &usecase.SystemRoleConflictInfo{
				ID:   "r-admin",
				Name: "admin",
			},
		},
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), updateRoleMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	payload, _ := data["updateRole"].(map[string]any)
	if payload == nil {
		t.Fatalf("expected data.updateRole, got nil; response: %v", resp)
	}
	if payload["__typename"] != "CannotModifySystemRoleError" {
		t.Fatalf("expected __typename=CannotModifySystemRoleError, got %v", payload["__typename"])
	}
	if payload["roleId"] != "r-admin" {
		t.Fatalf("expected roleId=r-admin, got %v", payload["roleId"])
	}
	if payload["roleName"] != "admin" {
		t.Fatalf("expected roleName=admin, got %v", payload["roleName"])
	}
	msg, _ := payload["message"].(string)
	if msg == "" {
		t.Fatalf("expected non-empty message, got %q", msg)
	}
}

// TestResolver_UpdateRole_BadInput verifies that BAD_USER_INPUT with
// extensions.field="name" from the usecase propagates unchanged.
func TestResolver_UpdateRole_BadInput(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		updateErr: &ucerr.ValidationError{Field: "name", Message: "name must contain only lowercase letters, digits, '_' or '-'"},
	}
	srv := newAdminRoleSrv(mock)
	body := `{"query":"mutation { updateRole(id: \"r1\", name: \"INVALID NAME\") { __typename ... on UpdateRoleSuccess { role { id name } } ... on CannotModifySystemRoleError { message roleId roleName } } }"}`
	resp := gqlRequest(t, srv, authedCtx("admin"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeBadUserInput) {
		t.Fatalf("expected BAD_USER_INPUT, got %q; response: %v", code, resp)
	}

	errs, _ := resp["errors"].([]any)
	ext, _ := errs[0].(map[string]any)["extensions"].(map[string]any)
	field, _ := ext["field"].(string)
	if field != "name" {
		t.Fatalf("expected extensions.field=name, got %q; ext: %v", field, ext)
	}
}

// TestResolver_UpdateRole_XORInvariantViolation covers the defensive guard
// where the usecase returns an UpdateRoleOutcome with neither variant set.
// This must surface as INTERNAL — the resolver refuses to render an
// unselectable union value.
func TestResolver_UpdateRole_XORInvariantViolation(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		updateOutcome: usecase.UpdateRoleOutcome{}, // both variants nil
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), updateRoleMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeInternal) {
		t.Fatalf("expected INTERNAL_SERVER_ERROR, got %q; response: %v", code, resp)
	}
}

// ---------------------------------------------------------------------------
// Mutation.deleteRole tests
// ---------------------------------------------------------------------------

const deleteRoleMutation = `{"query":"mutation { deleteRole(id: \"r-editor\") }"}`

// TestResolver_DeleteRole_Success verifies that deleteRole returns true when
// the usecase succeeds (no error).
func TestResolver_DeleteRole_Success(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		deleteErr: nil,
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), deleteRoleMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	deleted, _ := data["deleteRole"].(bool)
	if !deleted {
		t.Fatalf("expected data.deleteRole == true, got %v", data["deleteRole"])
	}
}

// TestResolver_DeleteRole_Forbidden verifies that attempting to delete the
// system "admin" role returns FORBIDDEN.
func TestResolver_DeleteRole_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		deleteErr: &ucerr.ForbiddenError{Message: "cannot delete system role 'admin'"},
	}
	srv := newAdminRoleSrv(mock)
	body := `{"query":"mutation { deleteRole(id: \"r-admin\") }"}`
	resp := gqlRequest(t, srv, authedCtx("admin"), body)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q; response: %v", code, resp)
	}
}

// ---------------------------------------------------------------------------
// Query.role tests
// ---------------------------------------------------------------------------

// TestResolver_Role_NotFound_ReturnsNull verifies that Query.role returns
// data.role == null with no errors when the usecase returns (nil, nil). This
// matches the nullable schema field for missing roles.
func TestResolver_Role_NotFound_ReturnsNull(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		// getResult == nil and getErr == nil: usecase found no role but returned
		// no error (missing row maps to (nil, nil)).
		getResult: nil,
		getErr:    nil,
	}
	srv := newAdminRoleSrv(mock)
	body := `{"query":"{ role(id: \"missing-id\") { id name } }"}`
	resp := gqlRequest(t, srv, authedCtx("admin"), body)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("expected no errors for missing role, got %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected data field, got nil; response: %v", resp)
	}
	// role must be explicitly null, not missing.
	if _, exists := data["role"]; !exists {
		t.Fatalf("expected data.role key (null), got absent; response: %v", resp)
	}
	if data["role"] != nil {
		t.Fatalf("expected data.role == null, got %v", data["role"])
	}
}

// ---------------------------------------------------------------------------
// Query.roles tests
// ---------------------------------------------------------------------------

const rolesQuery = `{"query":"{ roles { id name } }"}`

// TestResolver_Roles_HappyPath verifies that Query.roles returns the expected
// role list when the AdminRoleUsecase.List succeeds.
func TestResolver_Roles_HappyPath(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		listResult: []*domain.Role{
			{ID: "r-admin", Name: "admin"},
			{ID: "r-general", Name: "general"},
		},
	}
	srv := newAdminRoleSrv(mock)
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

// TestResolver_Roles_Forbidden verifies that FORBIDDEN from the usecase
// propagates unchanged to the caller.
func TestResolver_Roles_Forbidden(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		listErr: &ucerr.ForbiddenError{Message: "admin only"},
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("non-admin"), rolesQuery)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q", code)
	}
}

// TestResolver_Roles_Empty verifies that Query.roles returns an empty slice
// (not null) when the usecase returns zero roles.
func TestResolver_Roles_Empty(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		listResult: []*domain.Role{},
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), rolesQuery)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	roles, _ := data["roles"].([]any)
	if len(roles) != 0 {
		t.Fatalf("expected 0 roles, got %d; response: %v", len(roles), resp)
	}
}
