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
			assertForbidden(t, err, "")

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
			assertUnauthenticated(t, err)
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
	assertValidationError(t, err, "name", "")
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
			assertValidationError(t, err, "name", "")
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
	assertValidationError(t, err, "name", "")
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
			assertValidationError(t, err, "name", "")
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
	assertCancelled(t, err)
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
	assertValidationError(t, err, "name", "")
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
// system admin role, and pushes the normalised new name to the repo. The
// happy-path outcome carries the renamed Role and a nil SystemRoleConflict.
func TestAdminRole_Update_Success(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult:   &domain.Role{ID: "r-1", Name: "moderator"},
		updateResult: &domain.Role{ID: "r-1", Name: "reviewer"},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	outcome, err := uc.Update(authedCtx("admin-1"), "r-1", "Reviewer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.SystemRoleConflict != nil {
		t.Fatalf("expected SystemRoleConflict==nil on happy path, got %+v",
			outcome.SystemRoleConflict)
	}
	if outcome.Role == nil || outcome.Role.Name != "reviewer" {
		t.Fatalf("outcome.Role = %+v, want role with name=reviewer", outcome.Role)
	}
	if roles.lastUpdateID != "r-1" || roles.lastUpdateName != "reviewer" {
		t.Fatalf("repo update args = (%q,%q), want (r-1, reviewer)",
			roles.lastUpdateID, roles.lastUpdateName)
	}
}

// TestAdminRole_Update_RenameSystemRoleConflict enforces the system-role
// guard for every name in the protected set. Renaming any of them must
// return UpdateRoleOutcome{SystemRoleConflict: ...} with a nil error (the
// refusal is surfaced as data, not as an error), and the repo Update must
// not be called. A negative case (a non-system role with a similar-looking
// name) is included so a future widening of the guard cannot silently route
// a custom role through the system branch.
func TestAdminRole_Update_RenameSystemRoleConflict(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		roleName   string
		wantSystem bool
	}{
		{name: "admin", roleName: "admin", wantSystem: true},
		{name: "general", roleName: "general", wantSystem: true},
		// Negative: a non-system role with a confusable prefix must NOT be
		// rejected by the system-role guard. It still hits the repo Update.
		{name: "non-system 'general-2'", roleName: "general-2", wantSystem: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			roles := &mockAdminRoleRepoForCRUD{
				findResult: &domain.Role{ID: "r-x", Name: tc.roleName},
			}
			authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
			uc, _ := buildAdminRoleUC(roles, authChk)

			outcome, err := uc.Update(authedCtx("admin-1"), "r-x", "renamed")
			if tc.wantSystem {
				if err != nil {
					t.Fatalf("expected nil error on system-role refusal, got %v", err)
				}
				if outcome.SystemRoleConflict == nil {
					t.Fatalf("expected SystemRoleConflict, got nil; outcome=%+v", outcome)
				}
				if outcome.SystemRoleConflict.ID != "r-x" ||
					outcome.SystemRoleConflict.Name != tc.roleName {
					t.Fatalf("SystemRoleConflict = %+v, want {ID:r-x Name:%s}",
						outcome.SystemRoleConflict, tc.roleName)
				}
				if outcome.Role != nil {
					t.Fatalf("expected Role==nil on system-role refusal, got %+v", outcome.Role)
				}
				if roles.updateCalls != 0 {
					t.Fatalf("expected 0 repo update calls on system-role guard, got %d",
						roles.updateCalls)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for non-system role: %v", err)
			}
			if outcome.Role == nil {
				t.Fatalf("expected outcome.Role for non-system role, got nil")
			}
			if outcome.SystemRoleConflict != nil {
				t.Fatalf("expected SystemRoleConflict==nil for non-system role, got %+v",
					outcome.SystemRoleConflict)
			}
			if roles.updateCalls != 1 {
				t.Fatalf("expected 1 repo update call for non-system role, got %d",
					roles.updateCalls)
			}
		})
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
	assertValidationError(t, err, "id", "")
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
	assertValidationError(t, err, "name", "")
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
	assertValidationError(t, err, "name", "")
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
	assertValidationError(t, err, "id", "")
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

// TestAdminRole_Delete_SystemRoleForbidden enforces the system-role guard
// for every name in the protected set on Delete. Deleting any of them must
// be rejected with FORBIDDEN. A negative case (a non-system role with a
// confusable prefix) verifies the guard does not over-match.
func TestAdminRole_Delete_SystemRoleForbidden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		roleName string
		want     string
	}{
		{name: "admin", roleName: "admin", want: "FORBIDDEN"},
		{name: "general", roleName: "general", want: "FORBIDDEN"},
		// Negative: a non-system role with a confusable prefix must NOT be
		// rejected by the guard. It hits the repo Delete instead.
		{name: "non-system 'admin-2'", roleName: "admin-2", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			roles := &mockAdminRoleRepoForCRUD{
				findResult: &domain.Role{ID: "r-x", Name: tc.roleName},
			}
			authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
			uc, _ := buildAdminRoleUC(roles, authChk)

			err := uc.Delete(authedCtx("admin-1"), "r-x")
			if tc.want == "FORBIDDEN" {
				assertForbidden(t, err, "")
				if roles.deleteCalls != 0 {
					t.Fatalf("expected 0 repo delete calls on system-role guard, got %d",
						roles.deleteCalls)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for non-system role: %v", err)
			}
			if roles.deleteCalls != 1 {
				t.Fatalf("expected 1 repo delete call for non-system role, got %d",
					roles.deleteCalls)
			}
		})
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
	assertValidationError(t, err, "id", "")
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
	assertValidationError(t, err, "id", "")
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
	assertInternalChain(t, err, "usecase: admin role delete")
}

// Test 3: Update default-branch chain assertion.
// TestAdminRole_Update_RepoInternalError surfaces a generic repository error
// from the roles.Update call (anything other than the classified sentinels) as
// INTERNAL with the eris chain attached. The pattern mirrors
// TestAdminRole_Delete_RepoInternalError: requireAdmin OK, validateRoleName OK,
// FindByID returns a non-system role, roles.Update returns a non-sentinel error.
func TestAdminRole_Update_RepoInternalError(t *testing.T) {
	t.Parallel()

	roles := &mockAdminRoleRepoForCRUD{
		findResult: &domain.Role{ID: "r-1", Name: "moderator"}, // non-system role
		updateErr:  errors.New("boom: db blew up"),
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _ := buildAdminRoleUC(roles, authChk)

	outcome, err := uc.Update(authedCtx("admin-1"), "r-1", "reviewer")

	// The outcome struct must be zero-value (no Role, no SystemRoleConflict).
	if outcome.Role != nil || outcome.SystemRoleConflict != nil {
		t.Fatalf("expected zero-value outcome, got %+v", outcome)
	}
	// The error must be non-nil and carry the "usecase: admin role update" wrap.
	assertInternalChain(t, err, "usecase: admin role update")
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
