package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"backend/internal/domain"
	"backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// mockAdminRoleRepoForCRUD implements the narrow adminRoleRepoForCRUD surface.
// Each method records the args it was called with, plus a per-method return
// slot the tests pre-load. Counters let assertions confirm the usecase did
// (or did not) reach the repository.
type mockAdminRoleRepoForCRUD struct {
	// FindByID
	findResult *domain.Role
	findErr    error
	findCalls  int
	lastFindID string

	// Create
	createResult   *domain.Role
	createErr      error
	createCalls    int
	lastCreateName string

	// Update
	updateResult   *domain.Role
	updateErr      error
	updateCalls    int
	lastUpdateID   string
	lastUpdateName string

	// Delete
	deleteErr    error
	deleteCalls  int
	lastDeleteID string

	// ListAll
	listResult []*domain.Role
	listErr    error
	listCalls  int
}

func (m *mockAdminRoleRepoForCRUD) FindByID(_ context.Context, id string) (*domain.Role, error) {
	m.findCalls++
	m.lastFindID = id
	if m.findErr != nil {
		return nil, m.findErr
	}
	return m.findResult, nil
}

func (m *mockAdminRoleRepoForCRUD) Create(_ context.Context, name string) (*domain.Role, error) {
	m.createCalls++
	m.lastCreateName = name
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.createResult != nil {
		return m.createResult, nil
	}
	// Echo the normalised name so callers see what reached the repo.
	return &domain.Role{ID: "r-new", Name: name}, nil
}

func (m *mockAdminRoleRepoForCRUD) Update(_ context.Context, id, name string) (*domain.Role, error) {
	m.updateCalls++
	m.lastUpdateID = id
	m.lastUpdateName = name
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	if m.updateResult != nil {
		return m.updateResult, nil
	}
	return &domain.Role{ID: id, Name: name}, nil
}

func (m *mockAdminRoleRepoForCRUD) Delete(_ context.Context, id string) error {
	m.deleteCalls++
	m.lastDeleteID = id
	return m.deleteErr
}

func (m *mockAdminRoleRepoForCRUD) ListAll(_ context.Context) ([]*domain.Role, error) {
	m.listCalls++
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listResult, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildAdminRoleUC wires a usecase with the supplied stubs. nil arguments are
// replaced with empty defaults so individual tests need only specify what
// they exercise.
func buildAdminRoleUC(
	roles *mockAdminRoleRepoForCRUD,
	authChk AdminChecker,
) (AdminRoleUsecase, *mockAdminRoleRepoForCRUD) {
	if roles == nil {
		roles = &mockAdminRoleRepoForCRUD{}
	}
	if authChk == nil {
		authChk = &adminAuthChecker{}
	}
	return NewAdminRoleWithDeps(roles, authChk), roles
}

// ---------------------------------------------------------------------------
// FORBIDDEN gate: every method rejects non-admins
// ---------------------------------------------------------------------------

// TestAdminRole_NonAdminForbidden exercises the auth gate on every entry
// point. A non-admin caller must receive FORBIDDEN before any repository
// call is made.
func TestAdminRole_NonAdminForbidden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(uc AdminRoleUsecase) error
	}{
		{
			name: "List",
			call: func(uc AdminRoleUsecase) error {
				_, err := uc.List(authedCtx("u1"))
				return err
			},
		},
		{
			name: "Get",
			call: func(uc AdminRoleUsecase) error {
				_, err := uc.Get(authedCtx("u1"), "any")
				return err
			},
		},
		{
			name: "Create",
			call: func(uc AdminRoleUsecase) error {
				_, err := uc.Create(authedCtx("u1"), "moderator")
				return err
			},
		},
		{
			name: "Update",
			call: func(uc AdminRoleUsecase) error {
				_, err := uc.Update(authedCtx("u1"), "r-1", "moderator")
				return err
			},
		},
		{
			name: "Delete",
			call: func(uc AdminRoleUsecase) error {
				return uc.Delete(authedCtx("u1"), "r-1")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			authChk := &adminAuthChecker{admins: map[string]bool{}} // u1 is not admin
			roles := &mockAdminRoleRepoForCRUD{}
			uc, _ := buildAdminRoleUC(roles, authChk)

			err := tc.call(uc)
			assertGQLErr(t, err, "FORBIDDEN", "")

			total := roles.findCalls + roles.createCalls + roles.updateCalls +
				roles.deleteCalls + roles.listCalls
			if total != 0 {
				t.Fatalf("expected no repo calls, got find=%d create=%d update=%d delete=%d list=%d",
					roles.findCalls, roles.createCalls, roles.updateCalls,
					roles.deleteCalls, roles.listCalls)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// UNAUTHENTICATED gate: anonymous callers
// ---------------------------------------------------------------------------

// TestAdminRole_Unauthenticated covers the no-AuthUser-in-context branch on
// every entry point. Anonymous callers receive UNAUTHENTICATED and the
// IsAdmin checker is never consulted.
func TestAdminRole_Unauthenticated(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(uc AdminRoleUsecase) error
	}{
		{
			name: "List",
			call: func(uc AdminRoleUsecase) error {
				_, err := uc.List(anonCtx())
				return err
			},
		},
		{
			name: "Get",
			call: func(uc AdminRoleUsecase) error {
				_, err := uc.Get(anonCtx(), "any")
				return err
			},
		},
		{
			name: "Create",
			call: func(uc AdminRoleUsecase) error {
				_, err := uc.Create(anonCtx(), "moderator")
				return err
			},
		},
		{
			name: "Update",
			call: func(uc AdminRoleUsecase) error {
				_, err := uc.Update(anonCtx(), "r-1", "moderator")
				return err
			},
		},
		{
			name: "Delete",
			call: func(uc AdminRoleUsecase) error {
				return uc.Delete(anonCtx(), "r-1")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
			roles := &mockAdminRoleRepoForCRUD{}
			uc, _ := buildAdminRoleUC(roles, authChk)

			err := tc.call(uc)
			assertGQLErr(t, err, "UNAUTHENTICATED", "")
			if authChk.calls != 0 {
				t.Fatalf("expected 0 IsAdmin calls for anonymous caller, got %d", authChk.calls)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

// TestAdminRole_Create_Success normalises mixed-case input to lowercase and
// passes the canonical form to the repository.
func TestAdminRole_Create_Success(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		createResult: &domain.Role{ID: "r-new", Name: "moderator"},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	got, err := uc.Create(authedCtx("admin-1"), "Moderator")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Name != "moderator" {
		t.Fatalf("got = %+v, want role with name=moderator", got)
	}
	if roles.lastCreateName != "moderator" {
		t.Fatalf("repo create arg = %q, want %q (normalised)", roles.lastCreateName, "moderator")
	}
}

// TestAdminRole_Create_DuplicateName maps ErrRoleDuplicate from the repo to
// BAD_USER_INPUT keyed on "name".
func TestAdminRole_Create_DuplicateName(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{createErr: repository.ErrRoleDuplicate}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	_, err := uc.Create(authedCtx("admin-1"), "moderator")
	assertGQLErr(t, err, "BAD_USER_INPUT", "name")
}

// TestAdminRole_Create_EmptyName rejects whitespace-only input before the
// repository is reached.
func TestAdminRole_Create_EmptyName(t *testing.T) {
	t.Parallel()

	cases := []string{"", "   "}
	for _, in := range cases {
		t.Run("input="+in, func(t *testing.T) {
			t.Parallel()
			roles := &mockAdminRoleRepoForCRUD{}
			authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
			uc, _ := buildAdminRoleUC(roles, authChk)

			_, err := uc.Create(authedCtx("admin-1"), in)
			assertGQLErr(t, err, "BAD_USER_INPUT", "name")
			if roles.createCalls != 0 {
				t.Fatalf("expected no repo create on validation failure, got %d", roles.createCalls)
			}
		})
	}
}

// TestAdminRole_Create_TooLong rejects 51-character input before the repo
// is reached. The post-normalisation grapheme-cluster count is what bounds.
func TestAdminRole_Create_TooLong(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	overMax := strings.Repeat("a", roleNameMax+1)
	_, err := uc.Create(authedCtx("admin-1"), overMax)
	assertGQLErr(t, err, "BAD_USER_INPUT", "name")
	if roles.createCalls != 0 {
		t.Fatalf("expected no repo create on validation failure, got %d", roles.createCalls)
	}
}

// TestAdminRole_Create_InvalidChars rejects names whose post-normalisation
// form contains characters outside [a-z0-9_-]. Uppercase is folded by the
// normaliser (so "FOO" → "foo" passes), but a punctuation character that
// survives normalisation triggers the regex check.
func TestAdminRole_Create_InvalidChars(t *testing.T) {
	t.Parallel()

	cases := []string{
		"foo bar", // whitespace inside the name
		"FOO!",    // exclamation survives lower-case
		"rôle",    // non-ASCII (U+00F4 LATIN SMALL LETTER O WITH CIRCUMFLEX) outside the allowed set
		"foo.bar", // period
	}
	for _, in := range cases {
		t.Run("input="+in, func(t *testing.T) {
			t.Parallel()
			roles := &mockAdminRoleRepoForCRUD{}
			authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
			uc, _ := buildAdminRoleUC(roles, authChk)

			_, err := uc.Create(authedCtx("admin-1"), in)
			assertGQLErr(t, err, "BAD_USER_INPUT", "name")
			if roles.createCalls != 0 {
				t.Fatalf("expected no repo create on validation failure, got %d", roles.createCalls)
			}
		})
	}
}

// TestAdminRole_Create_ContextCanceled propagates context cancellation from
// the repository as CANCELLED rather than INTERNAL.
func TestAdminRole_Create_ContextCanceled(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{createErr: context.Canceled}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	_, err := uc.Create(authedCtx("admin-1"), "moderator")
	assertGQLErr(t, err, "CANCELLED", "")
}

// TestAdminRole_Create_NormalizesBeforeUniqueCheck exercises the normalisation
// (lowercase + trim) step so that mixed-case input reaches the repository in
// canonical form. The unique-check constraint then applies to the normalised
// name, not the original user input.
func TestAdminRole_Create_NormalizesBeforeUniqueCheck(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		wantRepo string
	}{
		{
			name:     "mixed case",
			input:    "Admin",
			wantRepo: "admin",
		},
		{
			name:     "uppercase",
			input:    "ADMIN",
			wantRepo: "admin",
		},
		{
			name:     "trim whitespace",
			input:    "  admin  ",
			wantRepo: "admin",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			roles := &mockAdminRoleRepoForCRUD{
				createResult: &domain.Role{ID: "r-new", Name: tc.wantRepo},
			}
			authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
			uc, _ := buildAdminRoleUC(roles, authChk)

			got, err := uc.Create(authedCtx("admin-1"), tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil || got.Name != tc.wantRepo {
				t.Fatalf("got = %+v, want role with name=%s", got, tc.wantRepo)
			}
			if roles.lastCreateName != tc.wantRepo {
				t.Fatalf("repo create arg = %q, want %q (normalised)", roles.lastCreateName, tc.wantRepo)
			}
			if roles.createCalls != 1 {
				t.Fatalf("expected 1 repo create call, got %d", roles.createCalls)
			}
		})
	}
}

// TestAdminRole_Create_DuplicateMatchesAfterNormalization ensures that when
// a user supplies a mixed-case name (e.g. "Admin") and a normalised form
// already exists in the system (e.g. a system role "admin"), the duplicate
// check catches the collision and returns BAD_USER_INPUT(field=name).
func TestAdminRole_Create_DuplicateMatchesAfterNormalization(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{createErr: repository.ErrRoleDuplicate}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	_, err := uc.Create(authedCtx("admin-1"), "Admin")
	assertGQLErr(t, err, "BAD_USER_INPUT", "name")
	if roles.lastCreateName != "admin" {
		t.Fatalf("repo create arg = %q, want %q (normalised before duplicate check)",
			roles.lastCreateName, "admin")
	}
	if roles.createCalls != 1 {
		t.Fatalf("expected 1 repo create call, got %d", roles.createCalls)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// TestAdminRole_Update_Success looks the role up, confirms it is not the
// system admin role, and pushes the normalised new name to the repo.
func TestAdminRole_Update_Success(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult:   &domain.Role{ID: "r-1", Name: "moderator"},
		updateResult: &domain.Role{ID: "r-1", Name: "reviewer"},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	got, err := uc.Update(authedCtx("admin-1"), "r-1", "Reviewer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Name != "reviewer" {
		t.Fatalf("got = %+v, want role with name=reviewer", got)
	}
	if roles.lastUpdateID != "r-1" || roles.lastUpdateName != "reviewer" {
		t.Fatalf("repo update args = (%q,%q), want (r-1, reviewer)",
			roles.lastUpdateID, roles.lastUpdateName)
	}
}

// TestAdminRole_Update_RenameAdminForbidden enforces the system-role guard:
// renaming the role whose current name is "admin" is rejected with FORBIDDEN.
// The repo Update must not be called.
func TestAdminRole_Update_RenameAdminForbidden(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult: &domain.Role{ID: "r-admin", Name: "admin"},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	_, err := uc.Update(authedCtx("admin-1"), "r-admin", "superadmin")
	assertGQLErr(t, err, "FORBIDDEN", "")
	if roles.updateCalls != 0 {
		t.Fatalf("expected 0 repo update calls on system-role guard, got %d", roles.updateCalls)
	}
}

// TestAdminRole_Update_NotFound maps ErrRoleNotFound from the FindByID lookup
// to BAD_USER_INPUT keyed on "id".
func TestAdminRole_Update_NotFound(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{findErr: repository.ErrRoleNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	_, err := uc.Update(authedCtx("admin-1"), "missing", "moderator")
	assertGQLErr(t, err, "BAD_USER_INPUT", "id")
	if roles.updateCalls != 0 {
		t.Fatalf("expected 0 repo update calls, got %d", roles.updateCalls)
	}
}

// TestAdminRole_Update_DuplicateOnRepo maps ErrRoleDuplicate from the repo
// Update call to BAD_USER_INPUT keyed on "name".
func TestAdminRole_Update_DuplicateOnRepo(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult: &domain.Role{ID: "r-1", Name: "moderator"},
		updateErr:  repository.ErrRoleDuplicate,
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	_, err := uc.Update(authedCtx("admin-1"), "r-1", "reviewer")
	assertGQLErr(t, err, "BAD_USER_INPUT", "name")
}

// TestAdminRole_Update_EmptyName rejects whitespace-only input before the
// FindByID lookup is issued. Validation runs first so the repo is never
// reached on a bad name.
func TestAdminRole_Update_EmptyName(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	_, err := uc.Update(authedCtx("admin-1"), "r-1", "   ")
	assertGQLErr(t, err, "BAD_USER_INPUT", "name")
	if roles.findCalls != 0 || roles.updateCalls != 0 {
		t.Fatalf("expected 0 repo calls on validation failure, got find=%d update=%d",
			roles.findCalls, roles.updateCalls)
	}
}

// TestAdminRole_Update_TOCTOUNotFound covers the race where FindByID succeeds
// (role existed at lookup time) but the subsequent Update returns
// ErrRoleNotFound because another admin deleted the row in between. This must
// surface as BAD_USER_INPUT(field=id), not INTERNAL.
func TestAdminRole_Update_TOCTOUNotFound(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult: &domain.Role{ID: "r-1", Name: "moderator"},
		updateErr:  repository.ErrRoleNotFound,
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	_, err := uc.Update(authedCtx("admin-1"), "r-1", "reviewer")
	assertGQLErr(t, err, "BAD_USER_INPUT", "id")
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// TestAdminRole_Delete_Success looks the role up, confirms it is not the
// system admin role, and asks the repo to delete it.
func TestAdminRole_Delete_Success(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult: &domain.Role{ID: "r-1", Name: "moderator"},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	if err := uc.Delete(authedCtx("admin-1"), "r-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if roles.deleteCalls != 1 {
		t.Fatalf("expected 1 repo delete call, got %d", roles.deleteCalls)
	}
	if roles.lastDeleteID != "r-1" {
		t.Fatalf("repo delete id = %q, want r-1", roles.lastDeleteID)
	}
}

// TestAdminRole_Delete_AdminForbidden enforces the system-role guard:
// deleting the role whose name is "admin" is rejected with FORBIDDEN. The
// repo Delete must not be called.
func TestAdminRole_Delete_AdminForbidden(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult: &domain.Role{ID: "r-admin", Name: "admin"},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	err := uc.Delete(authedCtx("admin-1"), "r-admin")
	assertGQLErr(t, err, "FORBIDDEN", "")
	if roles.deleteCalls != 0 {
		t.Fatalf("expected 0 repo delete calls on system-role guard, got %d", roles.deleteCalls)
	}
}

// TestAdminRole_Delete_NotFound maps ErrRoleNotFound from the FindByID lookup
// to BAD_USER_INPUT keyed on "id".
func TestAdminRole_Delete_NotFound(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{findErr: repository.ErrRoleNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	err := uc.Delete(authedCtx("admin-1"), "missing")
	assertGQLErr(t, err, "BAD_USER_INPUT", "id")
	if roles.deleteCalls != 0 {
		t.Fatalf("expected 0 repo delete calls, got %d", roles.deleteCalls)
	}
}

// TestAdminRole_Delete_TOCTOUNotFound covers the race where FindByID succeeds
// (role existed at lookup time) but the subsequent Delete returns
// ErrRoleNotFound because another admin deleted the row in between. This must
// surface as BAD_USER_INPUT(field=id), not INTERNAL.
func TestAdminRole_Delete_TOCTOUNotFound(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult: &domain.Role{ID: "r-1", Name: "moderator"},
		deleteErr:  repository.ErrRoleNotFound,
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	err := uc.Delete(authedCtx("admin-1"), "r-1")
	assertGQLErr(t, err, "BAD_USER_INPUT", "id")
}

// TestAdminRole_Delete_RepoInternalError surfaces a generic repository error
// (anything other than the classified sentinels) as INTERNAL with the eris
// chain attached.
func TestAdminRole_Delete_RepoInternalError(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult: &domain.Role{ID: "r-1", Name: "moderator"},
		deleteErr:  errors.New("boom: db blew up"),
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	err := uc.Delete(authedCtx("admin-1"), "r-1")
	assertGQLErr(t, err, "INTERNAL", "")
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// TestAdminRole_Get_NotFoundReturnsNilNil maps ErrRoleNotFound to (nil, nil)
// so the resolver renders Query.role(id) as null without erroring.
func TestAdminRole_Get_NotFoundReturnsNilNil(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{findErr: repository.ErrRoleNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	got, err := uc.Get(authedCtx("admin-1"), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil role, got %+v", got)
	}
}

// TestAdminRole_Get_Found returns the role when the repo finds it.
func TestAdminRole_Get_Found(t *testing.T) {
	t.Parallel()

	want := &domain.Role{ID: "r-1", Name: "moderator"}
	roles := &mockAdminRoleRepoForCRUD{findResult: want}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	got, err := uc.Get(authedCtx("admin-1"), "r-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got = %p, want %p", got, want)
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// TestAdminRole_List_Empty returns an empty (non-nil) slice when the repo has
// no rows. The repo contract says ListAll returns an empty slice; the usecase
// passes it through unchanged.
func TestAdminRole_List_Empty(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{listResult: []*domain.Role{}}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	got, err := uc.List(authedCtx("admin-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 roles, got %d", len(got))
	}
}

// TestAdminRole_List_Multiple passes the repo's ordered slice through
// untouched; the usecase does not re-sort.
func TestAdminRole_List_Multiple(t *testing.T) {
	t.Parallel()

	want := []*domain.Role{
		{ID: "r-admin", Name: "admin"},
		{ID: "r-general", Name: "general"},
		{ID: "r-reviewer", Name: "reviewer"},
	}
	roles := &mockAdminRoleRepoForCRUD{listResult: want}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	got, err := uc.List(authedCtx("admin-1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len(roles) = %d, want %d", len(got), len(want))
	}
	for i, r := range got {
		if r.ID != want[i].ID || r.Name != want[i].Name {
			t.Errorf("roles[%d] = {%q,%q}, want {%q,%q}", i, r.ID, r.Name, want[i].ID, want[i].Name)
		}
	}
}
