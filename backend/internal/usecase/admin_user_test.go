package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

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

	updateTxErr     error
	capturedTxPatch repository.UserUpdate
	updateTxCalls   int
	lastUpdateTxID  string

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

func (m *mockAdminUserRepository) UpdateTx(_ context.Context, _ *gorm.DB, id string, patch repository.UserUpdate) error {
	m.updateTxCalls++
	m.lastUpdateTxID = id
	m.capturedTxPatch = patch
	if m.updateTxErr != nil {
		return m.updateTxErr
	}
	if _, ok := m.users[id]; ok {
		return nil
	}
	return repository.ErrNotFound
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

// mockAdminRoleRepository implements the narrow adminRoleRepository surface
// (CRUD lookup only, after the RoleRepository split).
type mockAdminRoleRepository struct {
	// FindByIDs
	roles       map[string]*domain.Role
	findErr     error
	findCalls   int
	lastFindIDs []string
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

// mockAdminUserRoleRepository implements the narrow adminUserRoleRepository
// surface (membership write operations).
type mockAdminUserRoleRepository struct {
	setErr         error
	setCalls       int
	lastSetUID     string
	lastSetRoleIDs []string
}

func (m *mockAdminUserRoleRepository) SetUserRolesTx(_ context.Context, _ *gorm.DB, userID string, roleIDs []string) error {
	m.setCalls++
	m.lastSetUID = userID
	m.lastSetRoleIDs = append([]string(nil), roleIDs...)
	return m.setErr
}

// adminAuthChecker is a stand-alone AdminChecker stub for AdminUser tests. It
// is keyed on userID so the self-demotion test can return true for one caller
// and false for another. Tests pass it to buildAdminUC, which wraps it in
// NewAdminGate before constructing the usecase.
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
// they exercise. authChk is wrapped in NewAdminGate before being passed to
// the constructor.
func buildAdminUC(
	users *mockAdminUserRepository,
	roles *mockAdminRoleRepository,
	userRoles *mockAdminUserRoleRepository,
	authChk AdminChecker,
) (AdminUserUsecase, *mockAdminUserRepository, *mockAdminRoleRepository, *mockAdminUserRoleRepository) {
	return buildAdminUCWithTx(users, roles, userRoles, adminUserTxRunner(), authChk)
}

func buildAdminUCWithTx(
	users *mockAdminUserRepository,
	roles *mockAdminRoleRepository,
	userRoles *mockAdminUserRoleRepository,
	tx txRunner,
	authChk AdminChecker,
) (AdminUserUsecase, *mockAdminUserRepository, *mockAdminRoleRepository, *mockAdminUserRoleRepository) {
	if users == nil {
		users = &mockAdminUserRepository{}
	}
	if roles == nil {
		roles = &mockAdminRoleRepository{}
	}
	if userRoles == nil {
		userRoles = &mockAdminUserRoleRepository{}
	}
	if authChk == nil {
		authChk = &adminAuthChecker{}
	}
	uc := NewAdminUserWithDeps(users, roles, userRoles, tx, NewAdminGate(authChk), newTestLogger())
	return uc, users, roles, userRoles
}

func adminUserTxRunner() txRunner {
	return func(_ context.Context, fn func(tx *gorm.DB) error) error {
		return fn(&gorm.DB{})
	}
}

func countingAdminUserTxRunner() (txRunner, *int) {
	calls := 0
	return func(_ context.Context, fn func(tx *gorm.DB) error) error {
		calls++
		return fn(&gorm.DB{})
	}, &calls
}

// adminCallerCtx returns a context whose AuthUser sub is uid. Tests pair it
// with an adminAuthChecker whose admins map keys on the same uid.
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
			name: "EditUser",
			call: func(uc AdminUserUsecase) error {
				_, err := uc.EditUser(adminCallerCtx("u1"), "any", AdminEditUserInput{})
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			authChk := &adminAuthChecker{admins: map[string]bool{}} // u1 is not admin
			users := &mockAdminUserRepository{}
			userRoles := &mockAdminUserRoleRepository{}
			uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

			err := tc.call(uc)
			assertForbidden(t, err, "")

			if users.findCalls+users.updateTxCalls+users.listCalls != 0 {
				t.Fatalf("expected no user repo calls, got find=%d updateTx=%d list=%d",
					users.findCalls, users.updateTxCalls, users.listCalls)
			}
			if userRoles.setCalls != 0 {
				t.Fatalf("expected no role repo writes, got set=%d", userRoles.setCalls)
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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	stale := "00000000-0000-0000-0000-000000000000"
	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, &stale, nil, nil)
	assertValidationError(t, err, "after", "")
}

// TestAdminUser_List_BadCursor_Before mirrors the after case but for the
// backward-paging cursor field.
func TestAdminUser_List_BadCursor_Before(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{listErr: repository.ErrCursorNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	stale := "00000000-0000-0000-0000-000000000000"
	_, err := uc.List(adminCallerCtx("admin-1"), nil, intPtr(5), nil, &stale, nil)
	assertValidationError(t, err, "before", "")
}

// TestAdminUser_List_BothFirstAndLast covers the mutual-exclusion check on
// pagination args. Supplying both first and last is BAD_USER_INPUT.
func TestAdminUser_List_BothFirstAndLast(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(5), intPtr(5), nil, nil, nil)
	assertValidationError(t, err, "first", "")
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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(-1), nil, nil, nil, nil)
	assertValidationError(t, err, "first", "")
}

// TestAdminUser_List_FirstOverCap rejects values above the documented cap.
func TestAdminUser_List_FirstOverCap(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(adminUserMaxPageSize+1), nil, nil, nil, nil)
	assertValidationError(t, err, "first", "")
}

// TestAdminUser_List_AfterAndBeforeMutuallyExclusive verifies that supplying
// both cursor sides is rejected before the repository is reached.
func TestAdminUser_List_AfterAndBeforeMutuallyExclusive(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	after := "u-a"
	before := "u-b"
	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, &after, &before, nil)
	assertValidationError(t, err, "after", "")
	if users.listCalls != 0 {
		t.Fatalf("expected no repo call on cross-cursor rejection, got %d", users.listCalls)
	}
}

// TestAdminUser_List_FirstWithBefore rejects pairing forward count with the
// backward cursor — the page boundary would otherwise be ambiguous.
func TestAdminUser_List_FirstWithBefore(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	before := "u-b"
	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(5), nil, nil, &before, nil)
	assertValidationError(t, err, "before", "")
	if users.listCalls != 0 {
		t.Fatalf("expected no repo call, got %d", users.listCalls)
	}
}

// TestAdminUser_List_LastWithAfter rejects pairing backward count with the
// forward cursor.
func TestAdminUser_List_LastWithAfter(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	after := "u-a"
	_, err := uc.List(adminCallerCtx("admin-1"), nil, intPtr(5), &after, nil, nil)
	assertValidationError(t, err, "after", "")
	if users.listCalls != 0 {
		t.Fatalf("expected no repo call, got %d", users.listCalls)
	}
}

// TestAdminUser_List_BeforeWithoutLast rejects supplying a before cursor with
// no companion last value. Without last, the server cannot determine page size
// or direction, so the request is ambiguous and must be rejected.
func TestAdminUser_List_BeforeWithoutLast(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	before := "u-b"
	// No first, no last — only before.
	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, nil, &before, nil)
	assertValidationError(t, err, "before", "")
	if users.listCalls != 0 {
		t.Fatalf("expected no repo call on before-without-last, got %d", users.listCalls)
	}
}

// TestAdminUser_List_AfterWithoutFirst rejects supplying an after cursor with
// no companion first value. Without first, the server cannot determine page
// size or direction, so the request must be rejected.
func TestAdminUser_List_AfterWithoutFirst(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	after := "u-a"
	// No first, no last — only after.
	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, &after, nil, nil)
	assertValidationError(t, err, "after", "")
	if users.listCalls != 0 {
		t.Fatalf("expected no repo call on after-without-first, got %d", users.listCalls)
	}
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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	got, err := uc.Get(adminCallerCtx("admin-1"), "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil user, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// EditUser
// ---------------------------------------------------------------------------

func TestAdminUser_EditUser_ProfileAndRolesHappy(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "u-target", DisplayName: dnPtr("Bob")}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": target}}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		DisplayName: ptr("  Bob  "),
		Bio:         ptr(""),
		RoleIDs:     []string{"r-admin", "r-general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.User != target {
		t.Fatalf("outcome.User = %p, want %p", outcome.User, target)
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
	if users.updateTxCalls != 1 {
		t.Fatalf("UpdateTx calls = %d, want 1", users.updateTxCalls)
	}
	if users.capturedTxPatch.DisplayName == nil || *users.capturedTxPatch.DisplayName != "Bob" {
		t.Fatalf("DisplayName patch = %v, want Bob", users.capturedTxPatch.DisplayName)
	}
	if users.capturedTxPatch.Bio == nil || *users.capturedTxPatch.Bio != "" {
		t.Fatalf("Bio patch = %v, want explicit empty string", users.capturedTxPatch.Bio)
	}
	if userRoles.setCalls != 1 {
		t.Fatalf("SetUserRolesTx calls = %d, want 1", userRoles.setCalls)
	}
	if userRoles.lastSetUID != "u-target" {
		t.Fatalf("SetUserRolesTx userID = %q, want u-target", userRoles.lastSetUID)
	}
	assertStringSliceEqual(t, userRoles.lastSetRoleIDs, []string{"r-admin", "r-general"})
	if users.findCalls != 1 || users.lastFindID != "u-target" {
		t.Fatalf("refetch calls/id = %d/%q, want 1/u-target", users.findCalls, users.lastFindID)
	}
}

func TestAdminUser_EditUser_RolesOnlySkipsProfileUpdate(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "u-target"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": target}}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.User != target {
		t.Fatalf("outcome.User = %p, want %p", outcome.User, target)
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
	if users.updateTxCalls != 0 {
		t.Fatalf("UpdateTx calls = %d, want 0", users.updateTxCalls)
	}
	if userRoles.setCalls != 1 {
		t.Fatalf("SetUserRolesTx calls = %d, want 1", userRoles.setCalls)
	}
	assertStringSliceEqual(t, userRoles.lastSetRoleIDs, []string{"r-general"})
}

func TestAdminUser_EditUser_EmptyRoleIDsAllowedForOtherUser(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "u-target"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": target}}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.User != target {
		t.Fatalf("outcome.User = %p, want %p", outcome.User, target)
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
	if userRoles.setCalls != 1 {
		t.Fatalf("SetUserRolesTx calls = %d, want 1", userRoles.setCalls)
	}
	if len(userRoles.lastSetRoleIDs) != 0 {
		t.Fatalf("roleIDs = %v, want empty final set", userRoles.lastSetRoleIDs)
	}
}

func TestAdminUser_EditUser_DuplicateRoleIDsValidation(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general", "r-general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.Validation == nil || outcome.Validation.Field != "roleIds" {
		t.Fatalf("outcome.Validation = %+v, want field=roleIds", outcome.Validation)
	}
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0", *txCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0", userRoles.setCalls)
	}
}

func TestAdminUser_EditUser_CannotRevokeOwnAdmin(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"admin-1": {ID: "admin-1"}}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-general": {ID: "r-general", Name: "general"},
		},
	}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, roles, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "admin-1", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if !outcome.CannotRevokeOwnAdmin {
		t.Fatalf("CannotRevokeOwnAdmin = false, want true")
	}
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0", *txCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0", userRoles.setCalls)
	}
}

func TestAdminUser_EditUser_SelfKeepingAdminAllowed(t *testing.T) {
	t.Parallel()

	caller := &domain.User{ID: "admin-1"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"admin-1": caller}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-admin":   {ID: "r-admin", Name: "admin"},
			"r-general": {ID: "r-general", Name: "general"},
		},
	}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, roles, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "admin-1", AdminEditUserInput{
		RoleIDs: []string{"r-admin", "r-general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.User != caller {
		t.Fatalf("outcome.User = %p, want %p", outcome.User, caller)
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
	if userRoles.setCalls != 1 {
		t.Fatalf("SetUserRolesTx calls = %d, want 1", userRoles.setCalls)
	}
}

func TestAdminUser_EditUser_RoleNotFoundValidation(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": {ID: "u-target"}}}
	userRoles := &mockAdminUserRoleRepository{setErr: repository.ErrRoleNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"missing-role"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.Validation == nil || outcome.Validation.Field != "roleIds" {
		t.Fatalf("outcome.Validation = %+v, want field=roleIds", outcome.Validation)
	}
}

func TestAdminUser_EditUser_CancelledFromRoleSet(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": {ID: "u-target"}}}
	userRoles := &mockAdminUserRoleRepository{setErr: context.Canceled}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("context.Canceled identity: got %T %v", err, err)
	}
	if outcome.User != nil || outcome.Validation != nil || outcome.CannotRevokeOwnAdmin {
		t.Fatalf("expected zero-value outcome on cancellation, got %+v", outcome)
	}
}

func TestAdminUser_EditUser_InfraErrorFromTx(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": {ID: "u-target"}}}
	userRoles := &mockAdminUserRoleRepository{setErr: errors.New("db down")}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	// Pin both the outer "tx" wrap and the inner "replace roles" sub-op wrap
	// so a regression that drops either is caught. assertInternalChain matches
	// any frame; the two calls together prove both wraps fire.
	assertInternalChain(t, err, "usecase: admin user edit: tx")
	assertInternalChain(t, err, "usecase: admin user edit: replace roles")
	if outcome.User != nil || outcome.Validation != nil || outcome.CannotRevokeOwnAdmin {
		t.Fatalf("expected zero-value outcome on infra error, got %+v", outcome)
	}
}

// TestAdminUser_EditUser_UpdateTxInfraError_PinsUpdateProfileWrap fires the
// profile branch of the tx callback and asserts that the inner per-sub-op
// wrap ("update profile") is present in the chain. Without this test, a
// future regression that drops the inner wrap on UpdateTx would still pass
// TestAdminUser_EditUser_InfraErrorFromTx because the outer "tx" frame
// continues to match.
func TestAdminUser_EditUser_UpdateTxInfraError_PinsUpdateProfileWrap(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{
		users:       map[string]*domain.User{"u-target": {ID: "u-target"}},
		updateTxErr: errors.New("db down"),
	}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		DisplayName: ptr("Alice"),
		RoleIDs:     []string{"r-general"},
	})
	assertInternalChain(t, err, "usecase: admin user edit: update profile")
	assertInternalChain(t, err, "usecase: admin user edit: tx")
}

// TestAdminUser_EditUser_UpdateTxCancelled pins the inner isContextDone
// short-circuit on the profile branch of the tx callback. context.Canceled
// must propagate as bare-identity (not wrapped), per
// pin-unwrapped-context-error-with-identity-check.
func TestAdminUser_EditUser_UpdateTxCancelled(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{
		users:       map[string]*domain.User{"u-target": {ID: "u-target"}},
		updateTxErr: context.Canceled,
	}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		DisplayName: ptr("Alice"),
		RoleIDs:     []string{"r-general"},
	})
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("context.Canceled identity (UpdateTx branch): got %T %v", err, err)
	}
}

// TestAdminUser_EditUser_DisplayNameTooLongValidation pins the display-name
// validation branch surfaced via outcome.Validation. The plan explicitly
// required the displayName-too-long case; once the validators move to the
// domain layer or change their return type, this test catches the misroute.
func TestAdminUser_EditUser_DisplayNameTooLongValidation(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	long := strings.Repeat("a", domain.DisplayNameMax+1)
	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		DisplayName: ptr(long),
		RoleIDs:     []string{"r-general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.Validation == nil || outcome.Validation.Field != "displayName" {
		t.Fatalf("outcome.Validation = %+v, want field=displayName", outcome.Validation)
	}
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0 (validation must short-circuit)", *txCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0", userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_BioTooLongValidation pins the bio validation branch
// surfaced via outcome.Validation. Symmetric to the displayName case.
func TestAdminUser_EditUser_BioTooLongValidation(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	long := strings.Repeat("a", domain.BioMax+1)
	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		Bio:     ptr(long),
		RoleIDs: []string{"r-general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.Validation == nil || outcome.Validation.Field != "bio" {
		t.Fatalf("outcome.Validation = %+v, want field=bio", outcome.Validation)
	}
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0 (validation must short-circuit)", *txCalls)
	}
}

// TestAdminUser_EditUser_EmptyStringRoleIDValidation pins the empty-string
// branch of normalizeAdminEditRoleIDs. Duplicates are covered separately by
// TestAdminUser_EditUser_DuplicateRoleIDsValidation.
func TestAdminUser_EditUser_EmptyStringRoleIDValidation(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general", ""},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.Validation == nil || outcome.Validation.Field != "roleIds" {
		t.Fatalf("outcome.Validation = %+v, want field=roleIds", outcome.Validation)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0", userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_UserNotFoundValidation covers the ErrUserNotFound
// branch of mapAdminEditMutationError. The SetUserRolesTx sentinel surfaces
// as an InputValidationError on field=id.
func TestAdminUser_EditUser_UserNotFoundValidation(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": {ID: "u-target"}}}
	userRoles := &mockAdminUserRoleRepository{setErr: repository.ErrUserNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.Validation == nil || outcome.Validation.Field != "id" {
		t.Fatalf("outcome.Validation = %+v, want field=id", outcome.Validation)
	}
}

// TestAdminUser_EditUser_Self_FindByIDsInfraError pins the new self-edit
// lookup branch: when callerID == id and roleIDs is non-empty, FindByIDs is
// invoked outside the transaction. A non-cancellation infra error must
// surface as INTERNAL with the documented wrap prefix.
func TestAdminUser_EditUser_Self_FindByIDsInfraError(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"admin-1": {ID: "admin-1"}}}
	roles := &mockAdminRoleRepository{findErr: errors.New("db down")}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, roles, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "admin-1", AdminEditUserInput{
		RoleIDs: []string{"r-admin"},
	})
	assertInternalChain(t, err, "usecase: admin user edit: lookup roles")
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0", *txCalls)
	}
	if outcome.User != nil || outcome.Validation != nil || outcome.CannotRevokeOwnAdmin {
		t.Fatalf("expected zero-value outcome on infra error, got %+v", outcome)
	}
}

// TestAdminUser_EditUser_Self_FindByIDsCancelled verifies that a cancelled
// context from the self-edit lookup propagates as CANCELLED, not INTERNAL.
func TestAdminUser_EditUser_Self_FindByIDsCancelled(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"admin-1": {ID: "admin-1"}}}
	roles := &mockAdminRoleRepository{findErr: context.Canceled}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, roles, userRoles, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "admin-1", AdminEditUserInput{
		RoleIDs: []string{"r-admin"},
	})
	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("context.Canceled identity: got %T %v", err, err)
	}
}

// TestAdminUser_EditUser_Self_UnknownRoleIDValidation pins the misclassified-
// outcome fix: when the self-edit roleIDs set contains an unknown id (and
// thus FindByIDs returns a partial map), the user receives an InputValidation
// error pointing at roleIds, NOT a CannotRevokeOwnAdmin outcome. Without the
// fix, the keepsAdmin loop would still report the missing role as a
// self-demotion attempt and surface the wrong banner.
func TestAdminUser_EditUser_Self_UnknownRoleIDValidation(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"admin-1": {ID: "admin-1"}}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-admin": {ID: "r-admin", Name: domain.AdminRoleName},
		},
	}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, roles, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "admin-1", AdminEditUserInput{
		RoleIDs: []string{"r-admin", "r-unknown"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.Validation == nil || outcome.Validation.Field != "roleIds" {
		t.Fatalf("outcome.Validation = %+v, want field=roleIds", outcome.Validation)
	}
	if outcome.CannotRevokeOwnAdmin {
		t.Fatalf("CannotRevokeOwnAdmin = true, want false (the issue is unknown role, not self-demotion)")
	}
	if *txCalls != 0 {
		t.Fatalf("tx calls = %d, want 0", *txCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0", userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_TxRunnerNotConfigured covers the defensive branch
// triggered when NewAdminUser is constructed with a nil *gorm.DB. EditUser
// surfaces a wrapped 'tx runner not configured' error rather than panicking.
func TestAdminUser_EditUser_TxRunnerNotConfigured(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	// Pass tx=nil explicitly to mirror NewAdminUser(db=nil) behaviour.
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, nil, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	assertInternalChain(t, err, "usecase: admin user edit: tx runner not configured")
}

// TestAdminUser_EditUser_RefetchUserDisappeared covers the unusual race
// between tx-commit and refetch: the row vanished after the mutation
// succeeded. The error chain preserves ErrNotFound for downstream checks.
func TestAdminUser_EditUser_RefetchUserDisappeared(t *testing.T) {
	t.Parallel()

	// users.users intentionally empty so FindByID after the tx returns ErrNotFound.
	users := &mockAdminUserRepository{users: map[string]*domain.User{}}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	// UpdateTx is skipped because no profile patch was supplied, so only
	// SetUserRolesTx runs in the tx, which succeeds. The refetch then hits
	// the empty store and returns ErrNotFound, which is wrapped with the
	// 'user disappeared' message.
	assertInternalChain(t, err, "usecase: admin user edit: refetch: user disappeared")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected repository.ErrNotFound in chain, got %T: %v", err, err)
	}
}

// TestAdminUser_EditUser_RefetchCancelled verifies that a cancelled context
// from refetchUser propagates as CANCELLED, not INTERNAL.
func TestAdminUser_EditUser_RefetchCancelled(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{findErr: context.Canceled}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	assertCancelled(t, err)
}

// TestAdminUser_EditUser_RefetchInfraError covers the generic-infra branch of
// refetchUser. The wrap pins the operation context so log readers can trace
// which sub-op failed.
func TestAdminUser_EditUser_RefetchInfraError(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{findErr: errors.New("db down")}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	assertInternalChain(t, err, "usecase: admin user edit: refetch")
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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, nil, nil, nil)
	assertCancelled(t, err)
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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, nil, nil, nil)
	assertCancelled(t, err)
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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	_, err := uc.List(anonCtx(), nil, nil, nil, nil, nil)
	assertUnauthenticated(t, err)
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
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	_, err := uc.List(adminCallerCtx("admin-1"), nil, nil, nil, nil, nil)
	assertInternalChain(t, err, "usecase: admin user: check admin")
}

// TestAdminUser_Get_Cancelled verifies that a cancelled context from the
// repository FindByID call propagates as CANCELLED rather than INTERNAL.
func TestAdminUser_Get_Cancelled(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{findErr: context.Canceled}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	_, err := uc.Get(adminCallerCtx("admin-1"), "u-target")
	assertCancelled(t, err)
}

// ---------------------------------------------------------------------------
// XOR-invariant outcome assertions
// ---------------------------------------------------------------------------

// assertAdminEditUserOutcomeXOR asserts that exactly one of User,
// Validation, or CannotRevokeOwnAdmin is set in the outcome.
func assertAdminEditUserOutcomeXOR(t *testing.T, outcome AdminEditUserOutcome) {
	t.Helper()
	set := 0
	if outcome.User != nil {
		set++
	}
	if outcome.Validation != nil {
		set++
	}
	if outcome.CannotRevokeOwnAdmin {
		set++
	}
	if set != 1 {
		t.Fatalf("AdminEditUserOutcome XOR: expected exactly one variant set, got %d (outcome=%+v)", set, outcome)
	}
}

func assertStringSliceEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice len = %d, want %d (got=%v want=%v)", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q (got=%v want=%v)", i, got[i], want[i], got, want)
		}
	}
}
