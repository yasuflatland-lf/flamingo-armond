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

// mockAdminUserRepository implements the narrow adminUserRepository surface.
// Each method returns caller-controlled fixtures; the captured fields let the
// tests assert what the usecase passed downstream.
type mockAdminUserRepository struct {
	// FindByID
	users      map[string]*domain.User
	findErr    error
	findCalls  int
	lastFindID string

	// Update
	updateResult  *domain.User
	updateErr     error
	capturedPatch repository.UserUpdate
	updateCalls   int
	lastUpdateID  string

	// ListPage
	listResult     []*domain.User
	listTotal      int64
	listErr        error
	listCalls      int
	lastListAfter  *string
	lastListBefore *string
	lastListFirst  int
	lastListLast   int
	lastListSearch *string
}

func (m *mockAdminUserRepository) FindByID(_ context.Context, id string) (*domain.User, error) {
	m.findCalls++
	m.lastFindID = id
	if m.findErr != nil {
		return nil, m.findErr
	}
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}

func (m *mockAdminUserRepository) Update(_ context.Context, id string, patch repository.UserUpdate) (*domain.User, error) {
	m.updateCalls++
	m.lastUpdateID = id
	m.capturedPatch = patch
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	if m.updateResult != nil {
		return m.updateResult, nil
	}
	// Default behaviour: return the seeded user (when present) so refetch-style
	// flows downstream get a non-nil row.
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, repository.ErrNotFound
}

func (m *mockAdminUserRepository) ListPage(
	_ context.Context,
	after, before *string,
	first, last int,
	search *string,
) ([]*domain.User, int64, error) {
	m.listCalls++
	m.lastListAfter = after
	m.lastListBefore = before
	m.lastListFirst = first
	m.lastListLast = last
	m.lastListSearch = search
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	return m.listResult, m.listTotal, nil
}

// mockAdminRoleRepository implements the narrow adminRoleRepository surface.
type mockAdminRoleRepository struct {
	// FindByIDs
	roles       map[string]*domain.Role
	findErr     error
	findCalls   int
	lastFindIDs []string

	// AssignToUser
	assignErr     error
	assignCalls   int
	lastAssignUID string
	lastAssignRID string

	// RevokeFromUser
	revokeErr     error
	revokeCalls   int
	lastRevokeUID string
	lastRevokeRID string

	// ListAll
	allRoles   []*domain.Role
	listAllErr error
}

func (m *mockAdminRoleRepository) FindByIDs(_ context.Context, ids []string) (map[string]*domain.Role, error) {
	m.findCalls++
	m.lastFindIDs = append([]string(nil), ids...)
	if m.findErr != nil {
		return nil, m.findErr
	}
	out := make(map[string]*domain.Role, len(ids))
	for _, id := range ids {
		if r, ok := m.roles[id]; ok {
			out[id] = r
		}
	}
	return out, nil
}

func (m *mockAdminRoleRepository) AssignToUser(_ context.Context, userID, roleID string) error {
	m.assignCalls++
	m.lastAssignUID = userID
	m.lastAssignRID = roleID
	return m.assignErr
}

func (m *mockAdminRoleRepository) RevokeFromUser(_ context.Context, userID, roleID string) error {
	m.revokeCalls++
	m.lastRevokeUID = userID
	m.lastRevokeRID = roleID
	return m.revokeErr
}

func (m *mockAdminRoleRepository) ListAll(_ context.Context) ([]*domain.Role, error) {
	if m.listAllErr != nil {
		return nil, m.listAllErr
	}
	return m.allRoles, nil
}

// adminAuthChecker is a stand-alone admin checker for AdminUser tests. It
// reuses the same shape as mockAdminChecker but is keyed on userID so the
// self-demotion test can return true for one caller and false for another.
type adminAuthChecker struct {
	admins map[string]bool
	err    error
	calls  int
}

func (a *adminAuthChecker) IsAdmin(_ context.Context, userID string) (bool, error) {
	a.calls++
	if a.err != nil {
		return false, a.err
	}
	return a.admins[userID], nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildAdminUC wires a usecase with the supplied stubs. nil arguments are
// replaced with empty defaults so individual tests need only specify what
// they exercise.
func buildAdminUC(
	users *mockAdminUserRepository,
	roles *mockAdminRoleRepository,
	authChk AdminChecker,
) (AdminUserUsecase, *mockAdminUserRepository, *mockAdminRoleRepository) {
	if users == nil {
		users = &mockAdminUserRepository{}
	}
	if roles == nil {
		roles = &mockAdminRoleRepository{}
	}
	if authChk == nil {
		authChk = &adminAuthChecker{}
	}
	uc := NewAdminUserWithDeps(users, roles, authChk)
	return uc, users, roles
}

// adminCallerCtx returns a context whose AuthUser sub matches a caller marked
// as admin in the supplied checker.
func adminCallerCtx(uid string) context.Context {
	return authedCtx(uid)
}

// ---------------------------------------------------------------------------
// FORBIDDEN gate: every method rejects non-admins
// ---------------------------------------------------------------------------

// TestAdminUser_NonAdminForbidden exercises the auth gate on every entry
// point. A non-admin caller must receive FORBIDDEN before any repository
// call is made.
func TestAdminUser_NonAdminForbidden(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		call func(uc AdminUserUsecase) error
	}{
		{
			name: "List",
			call: func(uc AdminUserUsecase) error {
				_, err := uc.List(adminCallerCtx("u1"), nil, nil, nil, nil, nil)
				return err
			},
		},
		{
			name: "Get",
			call: func(uc AdminUserUsecase) error {
				_, err := uc.Get(adminCallerCtx("u1"), "any")
				return err
			},
		},
		{
			name: "Update",
			call: func(uc AdminUserUsecase) error {
				_, err := uc.Update(adminCallerCtx("u1"), "any", AdminUpdateUserInput{})
				return err
			},
		},
		{
			name: "AssignRole",
			call: func(uc AdminUserUsecase) error {
				_, err := uc.AssignRole(adminCallerCtx("u1"), "u2", "r1")
				return err
			},
		},
		{
			name: "RevokeRole",
			call: func(uc AdminUserUsecase) error {
				_, err := uc.RevokeRole(adminCallerCtx("u1"), "u2", "r1")
				return err
			},
		},
		{
			name: "ListRoles",
			call: func(uc AdminUserUsecase) error {
				_, err := uc.ListRoles(adminCallerCtx("u1"))
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			authChk := &adminAuthChecker{admins: map[string]bool{}} // u1 is not admin
			users := &mockAdminUserRepository{}
			roles := &mockAdminRoleRepository{}
			uc, _, _ := buildAdminUC(users, roles, authChk)

			err := tc.call(uc)
			assertGQLErr(t, err, "FORBIDDEN", "")

			if users.findCalls+users.updateCalls+users.listCalls != 0 {
				t.Fatalf("expected no user repo calls, got find=%d update=%d list=%d",
					users.findCalls, users.updateCalls, users.listCalls)
			}
			if roles.assignCalls+roles.revokeCalls != 0 {
				t.Fatalf("expected no role repo writes, got assign=%d revoke=%d",
					roles.assignCalls, roles.revokeCalls)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// TestAdminUser_List_Page1 covers the default forward page-1 case: no cursor,
// no overflow, exactly N rows returned with no next page.
func TestAdminUser_List_Page1(t *testing.T) {
	t.Parallel()

	page := []*domain.User{
		{ID: "u-aaa"},
		{ID: "u-bbb"},
		{ID: "u-ccc"},
	}
	users := &mockAdminUserRepository{listResult: page, listTotal: 3}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	out, err := uc.List(adminCallerCtx("admin-1"), nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 3 {
		t.Fatalf("TotalCount = %d, want 3", out.TotalCount)
	}
	if len(out.Edges) != 3 {
		t.Fatalf("len(Edges) = %d, want 3", len(out.Edges))
	}
	for i, edge := range out.Edges {
		if edge.Cursor != page[i].ID {
			t.Fatalf("edge[%d].Cursor = %q, want %q", i, edge.Cursor, page[i].ID)
		}
		if edge.Node != page[i] {
			t.Fatalf("edge[%d].Node mismatch", i)
		}
	}
	if out.PageInfo.HasNextPage {
		t.Fatal("HasNextPage = true, want false (under-fill page)")
	}
	if out.PageInfo.HasPreviousPage {
		t.Fatal("HasPreviousPage = true, want false (no after cursor)")
	}
	if out.PageInfo.StartCursor == nil || *out.PageInfo.StartCursor != "u-aaa" {
		t.Fatalf("StartCursor = %v, want u-aaa", out.PageInfo.StartCursor)
	}
	if out.PageInfo.EndCursor == nil || *out.PageInfo.EndCursor != "u-ccc" {
		t.Fatalf("EndCursor = %v, want u-ccc", out.PageInfo.EndCursor)
	}
	// Default page size: usecase asks repo for 100+1=101 rows.
	if users.lastListFirst != adminUserMaxPageSize+1 {
		t.Fatalf("repo first arg = %d, want %d", users.lastListFirst, adminUserMaxPageSize+1)
	}
	if users.lastListLast != 0 {
		t.Fatalf("repo last arg = %d, want 0", users.lastListLast)
	}
}

// TestAdminUser_List_ForwardPage2 covers a forward (after) page that came
// back over-filled (first+1 rows). The trailing extra row drives
// HasNextPage=true and is trimmed; HasPreviousPage=true because an after
// cursor was supplied.
func TestAdminUser_List_ForwardPage2(t *testing.T) {
	t.Parallel()

	// first=2 → repo asked for 3; repo returns 3 rows so the trailing one
	// is trimmed and HasNextPage flips on.
	page := []*domain.User{
		{ID: "u-2"},
		{ID: "u-3"},
		{ID: "u-4"},
	}
	users := &mockAdminUserRepository{listResult: page, listTotal: 10}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	after := "u-1"
	out, err := uc.List(adminCallerCtx("admin-1"), intPtr(2), nil, &after, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Edges) != 2 {
		t.Fatalf("len(Edges) = %d, want 2 (trailing row trimmed)", len(out.Edges))
	}
	if out.Edges[0].Cursor != "u-2" || out.Edges[1].Cursor != "u-3" {
		t.Fatalf("edges = %v, want [u-2, u-3]", out.Edges)
	}
	if !out.PageInfo.HasNextPage {
		t.Fatal("HasNextPage = false, want true (extra row present)")
	}
	if !out.PageInfo.HasPreviousPage {
		t.Fatal("HasPreviousPage = false, want true (after cursor supplied)")
	}
	if users.lastListFirst != 3 {
		t.Fatalf("repo first arg = %d, want 3 (2+1)", users.lastListFirst)
	}
	if users.lastListAfter == nil || *users.lastListAfter != "u-1" {
		t.Fatalf("repo after arg = %v, want u-1", users.lastListAfter)
	}
}

// TestAdminUser_List_Backward covers a backward (last+before) page in display
// order. The repository returns rows already reversed; the usecase trims the
// extra leading row and flips HasPreviousPage.
func TestAdminUser_List_Backward(t *testing.T) {
	t.Parallel()

	// last=2 → repo asked for 3; repo returns 3 rows in display order.
	// The leading extra row is trimmed and HasPreviousPage flips on.
	page := []*domain.User{
		{ID: "u-prev"}, // extra leading row, will be trimmed
		{ID: "u-x"},
		{ID: "u-y"},
	}
	users := &mockAdminUserRepository{listResult: page, listTotal: 10}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	before := "u-z"
	out, err := uc.List(adminCallerCtx("admin-1"), nil, intPtr(2), nil, &before, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Edges) != 2 {
		t.Fatalf("len(Edges) = %d, want 2 (leading row trimmed)", len(out.Edges))
	}
	if out.Edges[0].Cursor != "u-x" || out.Edges[1].Cursor != "u-y" {
		t.Fatalf("edges = %v, want [u-x, u-y]", out.Edges)
	}
	if !out.PageInfo.HasPreviousPage {
		t.Fatal("HasPreviousPage = false, want true (extra leading row)")
	}
	if !out.PageInfo.HasNextPage {
		t.Fatal("HasNextPage = false, want true (before cursor supplied)")
	}
	if users.lastListLast != 3 {
		t.Fatalf("repo last arg = %d, want 3 (2+1)", users.lastListLast)
	}
	if users.lastListBefore == nil || *users.lastListBefore != "u-z" {
		t.Fatalf("repo before arg = %v, want u-z", users.lastListBefore)
	}
}

// TestAdminUser_List_BadCursor_After covers the cursor-not-found branch: the
// repository surfaces ErrCursorNotFound which the usecase translates to
// BAD_USER_INPUT keyed on the cursor field that was supplied.
func TestAdminUser_List_BadCursor_After(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{listErr: repository.ErrCursorNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	stale := "00000000-0000-0000-0000-000000000000"
	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, &stale, nil, nil)
	assertGQLErr(t, err, "BAD_USER_INPUT", "after")
}

// TestAdminUser_List_BadCursor_Before mirrors the after case but for the
// backward-paging cursor field.
func TestAdminUser_List_BadCursor_Before(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{listErr: repository.ErrCursorNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	stale := "00000000-0000-0000-0000-000000000000"
	_, err := uc.List(adminCallerCtx("admin-1"), nil, intPtr(5), nil, &stale, nil)
	assertGQLErr(t, err, "BAD_USER_INPUT", "before")
}

// TestAdminUser_List_BothFirstAndLast covers the mutual-exclusion check on
// pagination args. Supplying both first and last is BAD_USER_INPUT.
func TestAdminUser_List_BothFirstAndLast(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(5), intPtr(5), nil, nil, nil)
	assertGQLErr(t, err, "BAD_USER_INPUT", "first")
	if users.listCalls != 0 {
		t.Fatalf("expected no repo call, got %d", users.listCalls)
	}
}

// TestAdminUser_List_FirstNegative rejects negative first/last per the
// resolveAdminPageSize contract.
func TestAdminUser_List_FirstNegative(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(-1), nil, nil, nil, nil)
	assertGQLErr(t, err, "BAD_USER_INPUT", "first")
}

// TestAdminUser_List_FirstOverCap rejects values above the documented cap.
func TestAdminUser_List_FirstOverCap(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(adminUserMaxPageSize+1), nil, nil, nil, nil)
	assertGQLErr(t, err, "BAD_USER_INPUT", "first")
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

// TestAdminUser_Get_Found returns the row when the repo finds it.
func TestAdminUser_Get_Found(t *testing.T) {
	t.Parallel()

	want := &domain.User{ID: "u-target"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": want}}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	got, err := uc.Get(adminCallerCtx("admin-1"), "u-target")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got = %p, want %p", got, want)
	}
}

// TestAdminUser_Get_Missing maps ErrNotFound to (nil, nil) so the resolver
// renders the GraphQL field as null.
func TestAdminUser_Get_Missing(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{}}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	got, err := uc.Get(adminCallerCtx("admin-1"), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil user, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// TestAdminUser_Update_DisplayName persists the patched display name (with
// surrounding whitespace trimmed) and returns the refreshed user.
func TestAdminUser_Update_DisplayName(t *testing.T) {
	t.Parallel()

	updated := &domain.User{ID: "u-target", DisplayName: ptr("Bob")}
	users := &mockAdminUserRepository{updateResult: updated}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	got, err := uc.Update(adminCallerCtx("admin-1"), "u-target", AdminUpdateUserInput{
		DisplayName: ptr("  Bob  "),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != updated {
		t.Fatalf("got = %p, want %p", got, updated)
	}
	if users.capturedPatch.DisplayName == nil || *users.capturedPatch.DisplayName != "Bob" {
		t.Fatalf("repo patch DisplayName = %v, want Bob", users.capturedPatch.DisplayName)
	}
	if users.capturedPatch.Bio != nil {
		t.Fatalf("repo patch Bio = %v, want nil", users.capturedPatch.Bio)
	}
}

// TestAdminUser_Update_BioClear covers the explicit-clear branch: bio == &""
// is forwarded as a non-nil pointer to "".
func TestAdminUser_Update_BioClear(t *testing.T) {
	t.Parallel()

	updated := &domain.User{ID: "u-target"}
	users := &mockAdminUserRepository{updateResult: updated}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.Update(adminCallerCtx("admin-1"), "u-target", AdminUpdateUserInput{
		Bio: ptr(""),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if users.capturedPatch.Bio == nil {
		t.Fatal("expected non-nil Bio in patch (explicit clear), got nil")
	}
	if *users.capturedPatch.Bio != "" {
		t.Fatalf("Bio = %q, want \"\"", *users.capturedPatch.Bio)
	}
}

// TestAdminUser_Update_DisplayNameEmpty rejects an empty (post-trim) display
// name with BAD_USER_INPUT.
func TestAdminUser_Update_DisplayNameEmpty(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.Update(adminCallerCtx("admin-1"), "u-target", AdminUpdateUserInput{
		DisplayName: ptr("   "),
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "displayName")
	if users.updateCalls != 0 {
		t.Fatalf("expected no repo update on validation failure, got %d", users.updateCalls)
	}
}

// TestAdminUser_Update_DisplayNameOverMax rejects a 51-char display name.
func TestAdminUser_Update_DisplayNameOverMax(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	overMax := strings.Repeat("a", displayNameMax+1)
	_, err := uc.Update(adminCallerCtx("admin-1"), "u-target", AdminUpdateUserInput{
		DisplayName: ptr(overMax),
	})
	assertGQLErr(t, err, "BAD_USER_INPUT", "displayName")
	if users.updateCalls != 0 {
		t.Fatalf("expected no repo update on validation failure, got %d", users.updateCalls)
	}
}

// ---------------------------------------------------------------------------
// AssignRole
// ---------------------------------------------------------------------------

// TestAdminUser_AssignRole_Idempotent calls AssignRole twice with the same
// (userID, roleID) pair and asserts that the usecase issues two repo calls
// (the repo handles ON CONFLICT DO NOTHING internally) without surfacing an
// error to the caller.
func TestAdminUser_AssignRole_Idempotent(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "u-target", DisplayName: ptr("Carol")}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": target}}
	roles := &mockAdminRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, roles, authChk)

	for i := 0; i < 2; i++ {
		got, err := uc.AssignRole(adminCallerCtx("admin-1"), "u-target", "r-admin")
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
		if got != target {
			t.Fatalf("call %d: got = %p, want %p", i, got, target)
		}
	}
	if roles.assignCalls != 2 {
		t.Fatalf("expected 2 assign calls, got %d", roles.assignCalls)
	}
	if roles.lastAssignUID != "u-target" || roles.lastAssignRID != "r-admin" {
		t.Fatalf("repo assign args = (%q,%q), want (u-target, r-admin)",
			roles.lastAssignUID, roles.lastAssignRID)
	}
}

// ---------------------------------------------------------------------------
// RevokeRole
// ---------------------------------------------------------------------------

// TestAdminUser_RevokeRole_Happy covers a clean revoke: caller is an admin,
// target is someone else, role lookup is skipped, refetched user is returned.
func TestAdminUser_RevokeRole_Happy(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "u-victim"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-victim": target}}
	roles := &mockAdminRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, roles, authChk)

	got, err := uc.RevokeRole(adminCallerCtx("admin-1"), "u-victim", "r-some")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != target {
		t.Fatalf("got = %p, want %p", got, target)
	}
	if roles.revokeCalls != 1 {
		t.Fatalf("expected 1 revoke call, got %d", roles.revokeCalls)
	}
	// Self-demotion guard does not fire when caller != target, so the role
	// lookup must not have been issued.
	if roles.findCalls != 0 {
		t.Fatalf("expected 0 role lookups for non-self target, got %d", roles.findCalls)
	}
}

// TestAdminUser_RevokeRole_SelfAdminForbidden covers the self-demotion guard:
// an admin cannot revoke their own admin role. The role lookup must classify
// the role as the admin role (by name) before the FORBIDDEN is returned.
func TestAdminUser_RevokeRole_SelfAdminForbidden(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{
		"admin-1": {ID: "admin-1"},
	}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-admin": {ID: "r-admin", Name: "admin"},
		},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, roles, authChk)

	_, err := uc.RevokeRole(adminCallerCtx("admin-1"), "admin-1", "r-admin")
	assertGQLErr(t, err, "FORBIDDEN", "")
	if roles.revokeCalls != 0 {
		t.Fatalf("expected 0 revoke calls on self-demotion, got %d", roles.revokeCalls)
	}
	if roles.findCalls != 1 {
		t.Fatalf("expected 1 role lookup for self-demotion check, got %d", roles.findCalls)
	}
}

// TestAdminUser_RevokeRole_SelfNonAdminAllowed covers the "self-revoke is
// allowed when the role isn't admin" branch: the self-demotion guard names
// the admin role specifically, so revoking a different role from yourself
// must succeed.
func TestAdminUser_RevokeRole_SelfNonAdminAllowed(t *testing.T) {
	t.Parallel()

	caller := &domain.User{ID: "admin-1"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"admin-1": caller}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-other": {ID: "r-other", Name: "moderator"},
		},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, roles, authChk)

	got, err := uc.RevokeRole(adminCallerCtx("admin-1"), "admin-1", "r-other")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != caller {
		t.Fatalf("got = %p, want %p", got, caller)
	}
	if roles.revokeCalls != 1 {
		t.Fatalf("expected 1 revoke call, got %d", roles.revokeCalls)
	}
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

// TestAdminUser_List_CancelledFromAdminCheck covers the ctx-cancellation
// propagation path through the auth gate. A canceled context must surface
// CANCELLED, not INTERNAL.
func TestAdminUser_List_CancelledFromAdminCheck(t *testing.T) {
	t.Parallel()

	authChk := &adminAuthChecker{err: context.Canceled}
	users := &mockAdminUserRepository{}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, nil, nil, nil)
	assertGQLErr(t, err, "CANCELLED", "")
	if users.listCalls != 0 {
		t.Fatalf("expected no repo call after cancellation, got %d", users.listCalls)
	}
}

// TestAdminUser_List_CancelledFromRepo covers the cancellation path through
// the repository call (after admin check passes).
func TestAdminUser_List_CancelledFromRepo(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{listErr: context.DeadlineExceeded}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, nil, nil, nil)
	assertGQLErr(t, err, "CANCELLED", "")
}

// TestAdminUser_Update_CancelledFromRepo covers cancellation surfacing through
// the Update repo call.
func TestAdminUser_Update_CancelledFromRepo(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{updateErr: context.Canceled}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.Update(adminCallerCtx("admin-1"), "u-target", AdminUpdateUserInput{
		DisplayName: ptr("Alice"),
	})
	assertGQLErr(t, err, "CANCELLED", "")
}

// ---------------------------------------------------------------------------
// Auth gate edge cases
// ---------------------------------------------------------------------------

// TestAdminUser_AnonymousUnauthenticated covers the no-AuthUser-in-context
// branch. Anonymous callers receive UNAUTHENTICATED; the IsAdmin checker is
// never consulted.
func TestAdminUser_AnonymousUnauthenticated(t *testing.T) {
	t.Parallel()

	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	users := &mockAdminUserRepository{}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.List(anonCtx(), nil, nil, nil, nil, nil)
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if authChk.calls != 0 {
		t.Fatalf("expected 0 IsAdmin calls for anonymous caller, got %d", authChk.calls)
	}
}

// TestAdminUser_IsAdminInternalError covers the IsAdmin-returns-non-cancellation-
// error branch. A DB-level error from the role check surfaces as INTERNAL.
func TestAdminUser_IsAdminInternalError(t *testing.T) {
	t.Parallel()

	authChk := &adminAuthChecker{err: errors.New("db down")}
	users := &mockAdminUserRepository{}
	uc, _, _ := buildAdminUC(users, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, nil, nil, nil)
	assertGQLErr(t, err, "INTERNAL", "")
}

// ---------------------------------------------------------------------------
// ListRoles
// ---------------------------------------------------------------------------

// TestAdminUser_ListRoles_NonAdmin verifies that a non-admin caller receives
// FORBIDDEN and the repository is never reached.
func TestAdminUser_ListRoles_NonAdmin(t *testing.T) {
	t.Parallel()

	authChk := &adminAuthChecker{admins: map[string]bool{}} // no admins
	roles := &mockAdminRoleRepository{}
	uc, _, _ := buildAdminUC(nil, roles, authChk)

	_, err := uc.ListRoles(adminCallerCtx("non-admin"))
	assertGQLErr(t, err, "FORBIDDEN", "")
}

// TestAdminUser_ListRoles_Admin_Empty verifies that an admin caller receives
// an empty (non-nil) slice when no roles exist.
func TestAdminUser_ListRoles_Admin_Empty(t *testing.T) {
	t.Parallel()

	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	roles := &mockAdminRoleRepository{allRoles: []*domain.Role{}}
	uc, _, _ := buildAdminUC(nil, roles, authChk)

	got, err := uc.ListRoles(adminCallerCtx("admin-1"))
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

// TestAdminUser_ListRoles_Admin_Multiple verifies that roles are passed through
// from the repository in name ASC order (the repo owns ordering).
func TestAdminUser_ListRoles_Admin_Multiple(t *testing.T) {
	t.Parallel()

	want := []*domain.Role{
		{ID: "r-admin", Name: "admin"},
		{ID: "r-general", Name: "general"},
		{ID: "r-reviewer", Name: "reviewer"},
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	roles := &mockAdminRoleRepository{allRoles: want}
	uc, _, _ := buildAdminUC(nil, roles, authChk)

	got, err := uc.ListRoles(adminCallerCtx("admin-1"))
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

// TestAdminUser_ListRoles_Cancelled verifies that a cancelled context
// propagates as CANCELLED rather than INTERNAL.
func TestAdminUser_ListRoles_Cancelled(t *testing.T) {
	t.Parallel()

	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	roles := &mockAdminRoleRepository{listAllErr: context.Canceled}
	uc, _, _ := buildAdminUC(nil, roles, authChk)

	_, err := uc.ListRoles(adminCallerCtx("admin-1"))
	assertGQLErr(t, err, "CANCELLED", "")
}
