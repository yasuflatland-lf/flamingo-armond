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
)

// ---------------------------------------------------------------------------
// mockAdminRoleUsecase — stub for AdminRoleUsecase
// ---------------------------------------------------------------------------

type mockAdminRoleUsecase struct {
	listResult   []*domain.Role
	listErr      error
	getResult    *domain.Role
	getErr       error
	createResult *domain.Role
	createErr    error
	updateResult *domain.Role
	updateErr    error
	deleteErr    error
}

func (m *mockAdminRoleUsecase) List(_ context.Context) ([]*domain.Role, error) {
	return m.listResult, m.listErr
}
func (m *mockAdminRoleUsecase) Get(_ context.Context, _ string) (*domain.Role, error) {
	return m.getResult, m.getErr
}
func (m *mockAdminRoleUsecase) Create(_ context.Context, _ string) (*domain.Role, error) {
	return m.createResult, m.createErr
}
func (m *mockAdminRoleUsecase) Update(_ context.Context, _, _ string) (*domain.Role, error) {
	return m.updateResult, m.updateErr
}
func (m *mockAdminRoleUsecase) Delete(_ context.Context, _ string) error {
	return m.deleteErr
}

// newAdminRoleSrv builds a gqlgen handler.Server backed by a mock
// AdminRoleUsecase. Other usecase fields are nil — only admin-role resolvers
// are exercised here.
func newAdminRoleSrv(roleUC usecase.AdminRoleUsecase) *handler.Server {
	r := resolver.NewResolver(nil, nil, nil, nil, nil, nil, nil, roleUC)
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: r}))
	srv.AddTransport(transport.POST{})
	return srv
}

// ---------------------------------------------------------------------------
// Mutation.createRole tests
// ---------------------------------------------------------------------------

const createRoleMutation = `{"query":"mutation { createRole(name: \"editor\") { id name } }"}`

// TestResolver_CreateRole_Success verifies that createRole returns the new
// role's id and name when the usecase succeeds.
func TestResolver_CreateRole_Success(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		createResult: &domain.Role{ID: "r-editor", Name: "editor"},
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("admin"), createRoleMutation)

	if _, hasErrs := resp["errors"]; hasErrs {
		t.Fatalf("unexpected errors: %v", resp["errors"])
	}
	data, _ := resp["data"].(map[string]any)
	role, _ := data["createRole"].(map[string]any)
	if role == nil {
		t.Fatalf("expected data.createRole, got nil; response: %v", resp)
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
		createErr: gqlerr.NewForbidden("admin only"),
	}
	srv := newAdminRoleSrv(mock)
	resp := gqlRequest(t, srv, authedCtx("non-admin"), createRoleMutation)

	code := errCode(t, resp)
	if code != string(gqlerr.CodeForbidden) {
		t.Fatalf("expected FORBIDDEN, got %q", code)
	}
}

// ---------------------------------------------------------------------------
// Mutation.updateRole tests
// ---------------------------------------------------------------------------

// TestResolver_UpdateRole_BadInput verifies that BAD_USER_INPUT with
// extensions.field="name" from the usecase propagates unchanged.
func TestResolver_UpdateRole_BadInput(t *testing.T) {
	t.Parallel()

	mock := &mockAdminRoleUsecase{
		updateErr: gqlerr.BadUserInput("name", "name must contain only lowercase letters, digits, '_' or '-'"),
	}
	srv := newAdminRoleSrv(mock)
	body := `{"query":"mutation { updateRole(id: \"r1\", name: \"INVALID NAME\") { id name } }"}`
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
		deleteErr: gqlerr.NewForbidden("cannot delete system role 'admin'"),
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
