package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gorm.io/gorm"

	"backend/internal/cursor"
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

	updateTxErr                 error
	capturedTxPatch             repository.UserUpdate
	updateTxCalls               int
	lastUpdateTxID              string
	lastUpdateTxExpectedVersion int64

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

	// DeleteAuthUserTx
	deleteAuthErr    error
	deleteAuthCalls  int
	lastDeleteAuthID string
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

func (m *mockAdminUserRepository) UpdateTxVersioned(_ context.Context, _ *gorm.DB, id string, patch repository.UserUpdate, expectedVersion int64) error {
	m.updateTxCalls++
	m.lastUpdateTxID = id
	m.capturedTxPatch = patch
	m.lastUpdateTxExpectedVersion = expectedVersion
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

func (m *mockAdminUserRepository) DeleteAuthUserTx(_ context.Context, _ *gorm.DB, id string) error {
	m.deleteAuthCalls++
	m.lastDeleteAuthID = id
	return m.deleteAuthErr
}

// mockAdminRoleRepository implements the narrow adminRoleRepository surface.
// The lookup is the tx-scoped, row-locking FindByIDsTx; the fake tx runner
// passes a non-nil *gorm.DB into the callback so the stub is reached on the
// in-transaction self-demotion-guard path.
type mockAdminRoleRepository struct {
	// FindByIDsTx
	roles       map[string]*domain.Role
	findErr     error
	findCalls   int
	lastFindIDs []string
}

func (m *mockAdminRoleRepository) FindByIDsTx(_ context.Context, _ *gorm.DB, ids []string) (map[string]*domain.Role, error) {
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

	// HasRoleTx — keyed by userID so DeleteUser tests can mark a specific target
	// as an admin (or not).
	hasRoleByUser map[string]bool
	hasRoleErr    error
	hasRoleCalls  int
	// lockCallsAtHasRole snapshots lockCalls the moment HasRoleTx runs, so a
	// test can prove the advisory lock was taken *before* the membership read
	// rather than merely before the count.
	lockCallsAtHasRole int

	// CountAdminsTx / AcquireAdminRoleLockTx
	adminCount int64
	countErr   error
	lockErr    error
	// lockCallsAtCount snapshots lockCalls the moment CountAdminsTx runs, so a
	// test can prove the advisory lock was taken *before* the count rather than
	// merely at some point during the request.
	lockCalls        int
	countCalls       int
	lockCallsAtCount int
}

func (m *mockAdminUserRoleRepository) SetUserRolesTx(_ context.Context, _ *gorm.DB, userID string, roleIDs []string) error {
	m.setCalls++
	m.lastSetUID = userID
	m.lastSetRoleIDs = append([]string(nil), roleIDs...)
	return m.setErr
}

func (m *mockAdminUserRoleRepository) HasRoleTx(_ context.Context, _ *gorm.DB, userID string, _ domain.RoleName) (bool, error) {
	m.hasRoleCalls++
	m.lockCallsAtHasRole = m.lockCalls
	if m.hasRoleErr != nil {
		return false, m.hasRoleErr
	}
	return m.hasRoleByUser[userID], nil
}

func (m *mockAdminUserRoleRepository) AcquireAdminRoleLockTx(_ context.Context, _ *gorm.DB) error {
	m.lockCalls++
	return m.lockErr
}

func (m *mockAdminUserRoleRepository) CountAdminsTx(_ context.Context, _ *gorm.DB) (int64, error) {
	m.countCalls++
	m.lockCallsAtCount = m.lockCalls
	return m.adminCount, m.countErr
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
	uc := newAdminUserWithDeps(users, roles, userRoles, tx, NewAdminGate(authChk), newTestLogger())
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
		{
			name: "DeleteUser",
			call: func(uc AdminUserUsecase) error {
				return uc.DeleteUser(adminCallerCtx("u1"), "any")
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

// TestAdminUserUsecase_DeleteUser exercises the DeleteUser guard chain and the
// happy path. The non-admin gate is covered by TestAdminUser_NonAdminForbidden;
// the cases here all use an admin caller.
func TestAdminUserUsecase_DeleteUser(t *testing.T) {
	t.Parallel()

	const caller = "admin-1"
	adminCaller := func() *adminAuthChecker {
		return &adminAuthChecker{admins: map[string]bool{caller: true}}
	}

	t.Run("self-deletion is forbidden", func(t *testing.T) {
		t.Parallel()
		users := &mockAdminUserRepository{}
		userRoles := &mockAdminUserRoleRepository{}
		uc, _, _, _ := buildAdminUC(users, nil, userRoles, adminCaller())

		err := uc.DeleteUser(adminCallerCtx(caller), caller)

		assertForbidden(t, err, "cannot delete your own account from the admin panel; use deleteMyAccount")
		if users.deleteAuthCalls != 0 {
			t.Fatalf("DeleteAuthUserTx must not run on self-deletion, got %d calls", users.deleteAuthCalls)
		}
		if userRoles.hasRoleCalls != 0 {
			t.Fatalf("HasRole must not run on self-deletion, got %d calls", userRoles.hasRoleCalls)
		}
	})

	t.Run("last admin is forbidden", func(t *testing.T) {
		t.Parallel()
		users := &mockAdminUserRepository{}
		userRoles := &mockAdminUserRoleRepository{
			hasRoleByUser: map[string]bool{"target": true},
			adminCount:    1,
		}
		uc, _, _, _ := buildAdminUC(users, nil, userRoles, adminCaller())

		err := uc.DeleteUser(adminCallerCtx(caller), "target")

		assertForbidden(t, err, "cannot delete the last admin account")
		if users.deleteAuthCalls != 0 {
			t.Fatalf("DeleteAuthUserTx must not run when the target is the last admin, got %d calls", users.deleteAuthCalls)
		}
	})

	t.Run("non-admin target is deleted without consulting the admin count", func(t *testing.T) {
		t.Parallel()
		users := &mockAdminUserRepository{}
		// "target" is absent from hasRoleByUser → not an admin. adminCount is set
		// to a blocking value to prove the count is never consulted for non-admins.
		userRoles := &mockAdminUserRoleRepository{adminCount: 1}
		uc, _, _, _ := buildAdminUC(users, nil, userRoles, adminCaller())

		err := uc.DeleteUser(adminCallerCtx(caller), "target")

		if err != nil {
			t.Fatalf("DeleteUser: unexpected error: %v", err)
		}
		if users.deleteAuthCalls != 1 || users.lastDeleteAuthID != "target" {
			t.Fatalf("DeleteAuthUserTx: calls=%d id=%q, want 1 and \"target\"", users.deleteAuthCalls, users.lastDeleteAuthID)
		}
	})

	t.Run("admin target that is not the last admin is deleted", func(t *testing.T) {
		t.Parallel()
		users := &mockAdminUserRepository{}
		userRoles := &mockAdminUserRoleRepository{
			hasRoleByUser: map[string]bool{"target": true},
			adminCount:    2,
		}
		uc, _, _, _ := buildAdminUC(users, nil, userRoles, adminCaller())

		err := uc.DeleteUser(adminCallerCtx(caller), "target")

		if err != nil {
			t.Fatalf("DeleteUser: unexpected error: %v", err)
		}
		if users.deleteAuthCalls != 1 {
			t.Fatalf("DeleteAuthUserTx: calls=%d, want 1", users.deleteAuthCalls)
		}
	})

	t.Run("missing user maps to validation error", func(t *testing.T) {
		t.Parallel()
		users := &mockAdminUserRepository{deleteAuthErr: repository.ErrNotFound}
		userRoles := &mockAdminUserRoleRepository{}
		uc, _, _, _ := buildAdminUC(users, nil, userRoles, adminCaller())

		err := uc.DeleteUser(adminCallerCtx(caller), "ghost")

		assertValidationError(t, err, "id", "user not found")
	})

	t.Run("infrastructure error wraps as internal chain", func(t *testing.T) {
		t.Parallel()
		users := &mockAdminUserRepository{deleteAuthErr: errors.New("boom")}
		userRoles := &mockAdminUserRoleRepository{}
		uc, _, _, _ := buildAdminUC(users, nil, userRoles, adminCaller())

		err := uc.DeleteUser(adminCallerCtx(caller), "target")

		assertInternalChain(t, err, "usecase: admin user: delete")
	})

	t.Run("context cancellation propagates", func(t *testing.T) {
		t.Parallel()
		users := &mockAdminUserRepository{}
		userRoles := &mockAdminUserRoleRepository{hasRoleErr: context.Canceled}
		uc, _, _, _ := buildAdminUC(users, nil, userRoles, adminCaller())

		err := uc.DeleteUser(adminCallerCtx(caller), "target")

		assertCancelled(t, err)
	})
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

// TestAdminUser_List_Page1 covers the default forward page-1 case: no cursor,
// no overflow, exactly N rows returned with no next page.
func TestAdminUser_List_Page1(t *testing.T) {
	t.Parallel()

	page := []*domain.User{
		{ID: domain.UserID("u-aaa")},
		{ID: domain.UserID("u-bbb")},
		{ID: domain.UserID("u-ccc")},
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
	if len(out.Users) != 3 {
		t.Fatalf("len(Users) = %d, want 3", len(out.Users))
	}
	for i, usr := range out.Users {
		if usr != page[i] {
			t.Fatalf("Users[%d] mismatch", i)
		}
	}
	if out.HasNext {
		t.Fatal("HasNext = true, want false (under-fill page)")
	}
	if out.HasPrev {
		t.Fatal("HasPrev = true, want false (no after cursor)")
	}
	// The usecase emits RAW ids in StartCur/EndCur; the resolver encodes once.
	if out.StartCur != "u-aaa" {
		t.Fatalf("StartCur = %q, want u-aaa", out.StartCur)
	}
	if out.EndCur != "u-ccc" {
		t.Fatalf("EndCur = %q, want u-ccc", out.EndCur)
	}
	// Default page size: usecase asks repo for 100+1=101 rows.
	if users.lastListFirst != maxPageSize+1 {
		t.Fatalf("repo first arg = %d, want %d", users.lastListFirst, maxPageSize+1)
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
		{ID: domain.UserID("u-2")},
		{ID: domain.UserID("u-3")},
		{ID: domain.UserID("u-4")},
	}
	users := &mockAdminUserRepository{listResult: page, listTotal: 10}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	after := "u-1"
	out, err := uc.List(adminCallerCtx("admin-1"), intPtr(2), nil, &after, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Users) != 2 {
		t.Fatalf("len(Users) = %d, want 2 (trailing row trimmed)", len(out.Users))
	}
	if string(out.Users[0].ID) != "u-2" || string(out.Users[1].ID) != "u-3" {
		t.Fatalf("users = %v, want [u-2, u-3]", out.Users)
	}
	if !out.HasNext {
		t.Fatal("HasNext = false, want true (extra row present)")
	}
	if !out.HasPrev {
		t.Fatal("HasPrev = false, want true (after cursor supplied)")
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
		{ID: domain.UserID("u-prev")}, // extra leading row, will be trimmed
		{ID: domain.UserID("u-x")},
		{ID: domain.UserID("u-y")},
	}
	users := &mockAdminUserRepository{listResult: page, listTotal: 10}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	before := "u-z"
	out, err := uc.List(adminCallerCtx("admin-1"), nil, intPtr(2), nil, &before, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Users) != 2 {
		t.Fatalf("len(Users) = %d, want 2 (leading row trimmed)", len(out.Users))
	}
	if string(out.Users[0].ID) != "u-x" || string(out.Users[1].ID) != "u-y" {
		t.Fatalf("users = %v, want [u-x, u-y]", out.Users)
	}
	if !out.HasPrev {
		t.Fatal("HasPrev = false, want true (extra leading row)")
	}
	if !out.HasNext {
		t.Fatal("HasNext = false, want true (before cursor supplied)")
	}
	if users.lastListLast != 3 {
		t.Fatalf("repo last arg = %d, want 3 (2+1)", users.lastListLast)
	}
	if users.lastListBefore == nil || *users.lastListBefore != "u-z" {
		t.Fatalf("repo before arg = %v, want u-z", users.lastListBefore)
	}
}

// TestAdminUser_List_BadCursor_After covers the cursor-not-found branch: a bare
// (decodable) after cursor passes cursor.Decode, reaches the repository, which
// surfaces ErrCursorNotFound; the usecase translates it to BAD_USER_INPUT keyed
// on the cursor field. first is supplied so the after/first relay check passes
// and the repository is actually reached.
func TestAdminUser_List_BadCursor_After(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{listErr: repository.ErrCursorNotFound}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	stale := "00000000-0000-0000-0000-000000000000"
	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(5), nil, &stale, nil, nil)
	assertValidationError(t, err, "after", "cursor not found")
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
	assertValidationError(t, err, "before", "cursor not found")
}

// TestAdminUser_List_MalformedCursor_After covers the decode-failure branch: a
// v1-enveloped after cursor with a malformed base64 payload is rejected as
// BAD_USER_INPUT ("invalid cursor") before the repository is consulted. A bare
// id is NOT a decode error — cursor.Decode passes it through and the repo's
// not-found path (BadCursor_After) handles it — so the corrupt fixture must use
// the v1 envelope to exercise this branch.
func TestAdminUser_List_MalformedCursor_After(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	after := "v1:!!!not-valid-base64!!!" // v1 prefix + malformed base64 payload
	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(5), nil, &after, nil, nil)
	assertValidationError(t, err, "after", "invalid cursor")
	if users.listCalls != 0 {
		t.Fatalf("malformed cursor must be rejected before ListPage, got %d calls", users.listCalls)
	}
}

// TestAdminUser_List_MalformedCursor_Before mirrors the after case for the
// backward-paging cursor field.
func TestAdminUser_List_MalformedCursor_Before(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

	before := "v1:!!!not-valid-base64!!!" // v1 prefix + malformed base64 payload
	_, err := uc.List(adminCallerCtx("admin-1"), nil, intPtr(5), nil, &before, nil)
	assertValidationError(t, err, "before", "invalid cursor")
	if users.listCalls != 0 {
		t.Fatalf("malformed cursor must be rejected before ListPage, got %d calls", users.listCalls)
	}
}

// TestAdminUser_List_V2Cursor_Rejected pins the guard against cross-envelope
// acceptance. The admin-users listing orders by the immutable created_at and
// never emits a v2 cursor, so a decodable v2 envelope — even one whose id names
// a real user — must be rejected before ListPage rather than paged by its raw
// id under ordering metadata this connection never validated. The fixture is a
// hand-built envelope because no production code path can produce one here.
func TestAdminUser_List_V2Cursor_Rejected(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		field string
	}{
		{name: "after", field: "after"},
		{name: "before", field: "before"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			users := &mockAdminUserRepository{}
			authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
			uc, _, _, _ := buildAdminUC(users, nil, nil, authChk)

			cur := cursor.EncodeV2(cursor.Payload{
				ID:        "00000000-0000-0000-0000-000000000000",
				OrderBy:   "sort_order",
				Direction: "ASC",
				OrderKey:  "1",
			})

			var err error
			if tc.field == "after" {
				_, err = uc.List(adminCallerCtx("admin-1"), intPtr(5), nil, &cur, nil, nil)
			} else {
				_, err = uc.List(adminCallerCtx("admin-1"), nil, intPtr(5), nil, &cur, nil)
			}
			assertValidationError(t, err, tc.field, "cursor does not match the requested ordering")
			if users.listCalls != 0 {
				t.Fatalf("v2 cursor must be rejected before ListPage, got %d calls", users.listCalls)
			}
		})
	}
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

	_, err := uc.List(adminCallerCtx("admin-1"), intPtr(maxPageSize+1), nil, nil, nil, nil)
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

	target := &domain.User{ID: "u-target", DisplayName: dnPtr("Bob"), Version: 41}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": target}}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		DisplayName:     ptr("  Bob  "),
		Bio:             ptr(""),
		RoleIDs:         []string{"r-admin", "r-general"},
		ExpectedVersion: 41,
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
		t.Fatalf("UpdateTxVersioned calls = %d, want 1", users.updateTxCalls)
	}
	if users.lastUpdateTxID != "u-target" {
		t.Fatalf("UpdateTxVersioned id = %q, want u-target", users.lastUpdateTxID)
	}
	if users.lastUpdateTxExpectedVersion != 41 {
		t.Fatalf("UpdateTxVersioned expectedVersion = %d, want 41", users.lastUpdateTxExpectedVersion)
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

func TestAdminUser_EditUser_RolesOnlyStillBumpsUserVersion(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "u-target", Version: 7}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": target}}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs:         []string{"r-general"},
		ExpectedVersion: 7,
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
		t.Fatalf("UpdateTxVersioned calls = %d, want 1", users.updateTxCalls)
	}
	if users.lastUpdateTxExpectedVersion != 7 {
		t.Fatalf("UpdateTxVersioned expectedVersion = %d, want 7", users.lastUpdateTxExpectedVersion)
	}
	if users.capturedTxPatch.DisplayName != nil || users.capturedTxPatch.Bio != nil || users.capturedTxPatch.AvatarURL != nil {
		t.Fatalf("UpdateTxVersioned patch = %+v, want empty patch for role-only edit", users.capturedTxPatch)
	}
	if userRoles.setCalls != 1 {
		t.Fatalf("SetUserRolesTx calls = %d, want 1", userRoles.setCalls)
	}
	assertStringSliceEqual(t, userRoles.lastSetRoleIDs, []string{"r-general"})
}

func TestAdminUser_EditUser_EmptyRoleIDsAllowedForOtherUser(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "u-target", Version: 9}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"u-target": target}}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		ExpectedVersion: 9,
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
		t.Fatalf("UpdateTxVersioned calls = %d, want 1", users.updateTxCalls)
	}
	if users.lastUpdateTxExpectedVersion != 9 {
		t.Fatalf("UpdateTxVersioned expectedVersion = %d, want 9", users.lastUpdateTxExpectedVersion)
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

// TestAdminUser_EditUser_RoleIDsExceedCapValidation pins the length-cap branch
// of normalizeAdminEditRoleIDs: a roleIds slice longer than maxAdminEditRoleIDs
// returns BAD_USER_INPUT on the "roleIds" field before any transaction opens, so
// the oversized slice never reaches FindByIDsTx / SetUserRolesTx. Mirrors
// TestCardUsecase_BulkDelete_RejectsTooManyIDs.
func TestAdminUser_EditUser_RoleIDsExceedCapValidation(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	ids := make([]string, maxAdminEditRoleIDs+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("r%d", i)
	}

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: ids,
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
	// The guard now runs at the front of the transaction (FOR UPDATE lock on
	// the role rows), so the tx runner is invoked once; the closure returns its
	// outcome via an early return nil before any write, so no write is committed.
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1 (guard runs inside tx, returns nil before any write)", *txCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0", userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_Self_NoRolesSubmitted_CannotRevokeOwnAdmin covers the
// zero-roleIDs sub-path of the self-demotion guard: when callerID == id and the
// submitted final role set is empty, FindByIDsTx is skipped (len(roleIDs) == 0),
// keepsAdmin stays false, and the guard captures its outcome and returns nil
// from the tx closure before any write. The guard outcome is returned without
// erroring — err is nil and the outcome carries CannotRevokeOwnAdmin.
func TestAdminUser_EditUser_Self_NoRolesSubmitted_CannotRevokeOwnAdmin(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"admin-1": {ID: "admin-1"}}}
	roles := &mockAdminRoleRepository{}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, roles, userRoles, tx, authChk)

	// Empty/nil RoleIDs: the final declarative role set is empty, so the caller
	// drops their own admin role.
	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "admin-1", AdminEditUserInput{
		RoleIDs: nil,
	})
	// The guard returns its outcome before any write; it must not surface as an
	// error to the caller.
	if err != nil {
		t.Fatalf("unexpected error (guard must not surface as an error): %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if !outcome.CannotRevokeOwnAdmin {
		t.Fatalf("CannotRevokeOwnAdmin = false, want true")
	}
	// The guard runs at the front of the transaction, so the tx runner is
	// invoked once; the closure returns nil before any write.
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1 (guard runs inside tx, returns nil before any write)", *txCalls)
	}
	// With zero roleIDs the locking lookup is skipped entirely.
	if roles.findCalls != 0 {
		t.Fatalf("FindByIDsTx calls = %d, want 0 (lookup skipped on empty role set)", roles.findCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0 (guard aborts before write)", userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_DemoteOther_LastAdmin exercises the demote-other
// branch of the last-admin guard: an admin submits a role set that drops the
// admin role from another user who is currently the only admin. The rejection
// carries the same shape as DeleteUser's last-admin rejection — a
// *ucerr.ForbiddenError on the error channel, not an outcome field — and no
// write reaches the repository.
func TestAdminUser_EditUser_DemoteOther_LastAdmin(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"target": {ID: "target"}}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-general": {ID: "r-general", Name: "general"},
		},
	}
	userRoles := &mockAdminUserRoleRepository{
		hasRoleByUser: map[string]bool{"target": true},
		adminCount:    1,
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, roles, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})

	assertForbidden(t, err, "cannot remove the admin role from the last admin account")
	if outcome.User != nil || outcome.Validation != nil || outcome.CannotRevokeOwnAdmin || outcome.ConcurrentUpdate {
		t.Fatalf("outcome must be zero when the guard rejects, got %+v", outcome)
	}
	if users.updateTxCalls != 0 {
		t.Fatalf("UpdateTxVersioned calls = %d, want 0 (guard aborts before any write)", users.updateTxCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0 (guard aborts before any write)", userRoles.setCalls)
	}
	// The count must be read while the advisory lock is held; otherwise two
	// mutual demotions each observe two admins and both commit.
	if userRoles.countCalls != 1 || userRoles.lockCallsAtCount != 1 {
		t.Fatalf("count=%d lockCallsAtCount=%d, want the admin count read once under the lock",
			userRoles.countCalls, userRoles.lockCallsAtCount)
	}
}

// TestAdminUser_EditUser_DemoteOther_NotLastAdmin proves the guard only blocks
// the last admin: with another admin left, the demotion goes through.
func TestAdminUser_EditUser_DemoteOther_NotLastAdmin(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "target"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"target": target}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-general": {ID: "r-general", Name: "general"},
		},
	}
	userRoles := &mockAdminUserRoleRepository{
		hasRoleByUser: map[string]bool{"target": true},
		adminCount:    2,
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, roles, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	if err != nil {
		t.Fatalf("EditUser: unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.User != target {
		t.Fatalf("outcome.User = %v, want %v", outcome.User, target)
	}
	if userRoles.setCalls != 1 {
		t.Fatalf("SetUserRolesTx calls = %d, want 1", userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_DemoteOther_NonAdminTargetSkipsCount proves the guard
// short-circuits for a target that does not hold the admin role: a blocking
// admin count is configured, and the edit still succeeds because the count is
// never consulted.
func TestAdminUser_EditUser_DemoteOther_NonAdminTargetSkipsCount(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "target"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"target": target}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-general": {ID: "r-general", Name: "general"},
		},
	}
	// "target" is absent from hasRoleByUser → not an admin. adminCount is set to
	// a blocking value to prove the count is never consulted.
	userRoles := &mockAdminUserRoleRepository{adminCount: 1}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, roles, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	if err != nil {
		t.Fatalf("EditUser: unexpected error: %v", err)
	}
	if outcome.User != target {
		t.Fatalf("outcome.User = %v, want %v", outcome.User, target)
	}
	if userRoles.countCalls != 0 {
		t.Fatalf("count=%d, want 0 for a non-admin target", userRoles.countCalls)
	}
	// The lock is still taken once: the membership read that decides the target
	// is a non-admin must itself happen under it, otherwise a target promoted
	// concurrently is read as a non-admin and skips the count entirely.
	if userRoles.lockCalls != 1 || userRoles.hasRoleCalls != 1 || userRoles.lockCallsAtHasRole != 1 {
		t.Fatalf("lock=%d hasRole=%d lockCallsAtHasRole=%d, want the membership read taken once under the lock",
			userRoles.lockCalls, userRoles.hasRoleCalls, userRoles.lockCallsAtHasRole)
	}
}

// TestAdminUser_EditUser_DemoteOther_KeepingAdminSkipsGuard proves the guard is
// keyed on the submitted set dropping admin, not on the edit touching an admin:
// a role set that retains admin never consults the count.
func TestAdminUser_EditUser_DemoteOther_KeepingAdminSkipsGuard(t *testing.T) {
	t.Parallel()

	target := &domain.User{ID: "target"}
	users := &mockAdminUserRepository{users: map[string]*domain.User{"target": target}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-admin": {ID: "r-admin", Name: "admin"},
		},
	}
	userRoles := &mockAdminUserRoleRepository{
		hasRoleByUser: map[string]bool{"target": true},
		adminCount:    1,
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, roles, userRoles, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "target", AdminEditUserInput{
		RoleIDs: []string{"r-admin"},
	})
	if err != nil {
		t.Fatalf("EditUser: unexpected error: %v", err)
	}
	if outcome.User != target {
		t.Fatalf("outcome.User = %v, want %v", outcome.User, target)
	}
	if userRoles.hasRoleCalls != 0 || userRoles.countCalls != 0 {
		t.Fatalf("hasRole=%d count=%d, want neither when the submitted set keeps admin",
			userRoles.hasRoleCalls, userRoles.countCalls)
	}
}

// TestAdminUser_EditUser_DemoteOther_CountAdminsInfraError pins the wrap prefix
// applied when the guard's count fails for an infrastructure reason, so the
// logged error_chain attributes the failure to the edit module.
func TestAdminUser_EditUser_DemoteOther_CountAdminsInfraError(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{users: map[string]*domain.User{"target": {ID: "target"}}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-general": {ID: "r-general", Name: "general"},
		},
	}
	userRoles := &mockAdminUserRoleRepository{
		hasRoleByUser: map[string]bool{"target": true},
		countErr:      errors.New("boom"),
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, roles, userRoles, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})

	assertInternalChain(t, err, "usecase: admin user edit: count admins")
	if users.updateTxCalls != 0 {
		t.Fatalf("UpdateTxVersioned calls = %d, want 0", users.updateTxCalls)
	}
}

// demoteOtherLockFailureUC wires the demote-other fixture with a failing
// AcquireAdminRoleLockTx so both lock-failure tests below share one setup.
func demoteOtherLockFailureUC(lockErr error) (AdminUserUsecase, *mockAdminUserRepository, *mockAdminUserRoleRepository) {
	users := &mockAdminUserRepository{users: map[string]*domain.User{"target": {ID: "target"}}}
	roles := &mockAdminRoleRepository{
		roles: map[string]*domain.Role{
			"r-general": {ID: "r-general", Name: "general"},
		},
	}
	userRoles := &mockAdminUserRoleRepository{
		hasRoleByUser: map[string]bool{"target": true},
		adminCount:    2,
		lockErr:       lockErr,
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, roles, userRoles, authChk)
	return uc, users, userRoles
}

// TestAdminUser_EditUser_DemoteOther_AcquireAdminRoleLockInfraError pins the
// fail-closed behaviour when the advisory lock cannot be taken: the request
// aborts before the membership read, before the count, and before any write.
// Swallowing the lock error and continuing would count without holding the
// lock, reopening the mutual-demotion race the lock exists to close.
func TestAdminUser_EditUser_DemoteOther_AcquireAdminRoleLockInfraError(t *testing.T) {
	t.Parallel()

	uc, users, userRoles := demoteOtherLockFailureUC(errors.New("boom"))

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})

	assertInternalChain(t, err, "usecase: admin user edit: count admins")
	if userRoles.hasRoleCalls != 0 || userRoles.countCalls != 0 {
		t.Fatalf("hasRole=%d count=%d, want neither once the lock could not be taken",
			userRoles.hasRoleCalls, userRoles.countCalls)
	}
	if users.updateTxCalls != 0 || userRoles.setCalls != 0 {
		t.Fatalf("updateTx=%d set=%d, want no write once the lock could not be taken",
			users.updateTxCalls, userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_DemoteOther_AcquireAdminRoleLockCancelled proves a
// cancelled context surfaces unwrapped from the lock-failure branch.
func TestAdminUser_EditUser_DemoteOther_AcquireAdminRoleLockCancelled(t *testing.T) {
	t.Parallel()

	uc, _, userRoles := demoteOtherLockFailureUC(context.Canceled)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})

	assertCancelled(t, err)
	if userRoles.countCalls != 0 {
		t.Fatalf("count = %d, want 0 once the lock could not be taken", userRoles.countCalls)
	}
}

// TestAdminUser_DeleteUser_AcquireAdminRoleLockInfraError is the DeleteUser
// twin of the EditUser lock-failure test above.
func TestAdminUser_DeleteUser_AcquireAdminRoleLockInfraError(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	userRoles := &mockAdminUserRoleRepository{
		hasRoleByUser: map[string]bool{"target": true},
		adminCount:    2,
		lockErr:       errors.New("boom"),
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	err := uc.DeleteUser(adminCallerCtx("admin-1"), "target")

	assertInternalChain(t, err, "usecase: admin user: delete: count admins")
	if userRoles.hasRoleCalls != 0 || userRoles.countCalls != 0 {
		t.Fatalf("hasRole=%d count=%d, want neither once the lock could not be taken",
			userRoles.hasRoleCalls, userRoles.countCalls)
	}
	if users.deleteAuthCalls != 0 {
		t.Fatalf("DeleteAuthUserTx calls = %d, want 0 once the lock could not be taken", users.deleteAuthCalls)
	}
}

// TestAdminUser_DeleteUser_MembershipReadTakenUnderLock pins the ordering the
// guard depends on: the target's admin membership is read only after the
// advisory lock is held. A read taken before the lock can observe a target that
// a concurrent request is about to promote, skipping the count entirely.
func TestAdminUser_DeleteUser_MembershipReadTakenUnderLock(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{}
	userRoles := &mockAdminUserRoleRepository{
		hasRoleByUser: map[string]bool{"target": true},
		adminCount:    2,
	}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	if err := uc.DeleteUser(adminCallerCtx("admin-1"), "target"); err != nil {
		t.Fatalf("DeleteUser: unexpected error: %v", err)
	}
	if userRoles.hasRoleCalls != 1 || userRoles.lockCallsAtHasRole != 1 {
		t.Fatalf("hasRole=%d lockCallsAtHasRole=%d, want the membership read taken once under the lock",
			userRoles.hasRoleCalls, userRoles.lockCallsAtHasRole)
	}
	if userRoles.lockCallsAtCount != 1 {
		t.Fatalf("lockCallsAtCount = %d, want the count read under the same lock", userRoles.lockCallsAtCount)
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
	// The locking FOR UPDATE lookup must be traversed before the write so the
	// keepsAdmin decision reads role names under the row lock.
	if roles.findCalls != 1 {
		t.Fatalf("FindByIDsTx calls = %d, want 1 (locking lookup before write)", roles.findCalls)
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

func TestAdminUser_EditUser_ConcurrentUpdateOutcome(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{
		users:       map[string]*domain.User{"u-target": {ID: "u-target", Version: 8}},
		updateTxErr: repository.ErrConcurrentUpdate,
	}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	tx, txCalls := countingAdminUserTxRunner()
	uc, _, _, _ := buildAdminUCWithTx(users, nil, userRoles, tx, authChk)

	outcome, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		DisplayName:     ptr("Alice"),
		RoleIDs:         []string{"r-general"},
		ExpectedVersion: 8,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if !outcome.ConcurrentUpdate {
		t.Fatalf("ConcurrentUpdate = false, want true")
	}
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1", *txCalls)
	}
	if users.updateTxCalls != 1 {
		t.Fatalf("UpdateTxVersioned calls = %d, want 1", users.updateTxCalls)
	}
	if users.lastUpdateTxExpectedVersion != 8 {
		t.Fatalf("UpdateTxVersioned expectedVersion = %d, want 8", users.lastUpdateTxExpectedVersion)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0 (conflict aborts before role write)", userRoles.setCalls)
	}
	if users.findCalls != 0 {
		t.Fatalf("FindByID calls = %d, want 0 (conflict skips refetch)", users.findCalls)
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

// TestAdminUser_EditUser_Self_FindByIDsInfraError pins the self-edit lookup
// branch: when callerID == id and roleIDs is non-empty, FindByIDsTx is invoked
// at the front of the transaction (FOR UPDATE lock). A non-cancellation infra
// error must surface as INTERNAL with the documented wrap prefix.
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
	// The lookup is now inside the transaction, so the tx runner is invoked
	// once; the infra error propagates out of the closure and rolls back.
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1 (lookup runs inside tx)", *txCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0 (lookup failed before write)", userRoles.setCalls)
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
// thus FindByIDsTx returns a partial map), the user receives an InputValidation
// error pointing at roleIds, NOT a CannotRevokeOwnAdmin outcome. Without the
// fix, the keepsAdmin loop would still report the missing role as a
// self-demotion attempt and surface the wrong banner. The partial-map check now
// runs inside the transaction via the FOR UPDATE lookup, so the guard captures
// its outcome and returns nil from the tx closure before any write.
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
	// The partial-map check runs inside the transaction, so the tx runner is
	// invoked once; the guard returns nil from the closure before any write.
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1 (guard runs inside tx, returns nil before any write)", *txCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0", userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_Self_UnknownRoleViaTxLookup proves the unknown-role
// self-edit path (len(roles) != len(roleIDs)) is driven by the in-transaction,
// row-locking FindByIDsTx lookup. The role stub must be reached exactly once
// via the tx-scoped lookup, the partial map must yield a roleIds validation
// outcome, and the guard that returns nil from the tx closure must never
// surface as an error from EditUser (err is nil; the outcome carries the result).
func TestAdminUser_EditUser_Self_UnknownRoleViaTxLookup(t *testing.T) {
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
		RoleIDs: []string{"r-admin", "r-missing"},
	})
	// The guard returns its outcome before any write; it must not surface as an
	// error to the caller.
	if err != nil {
		t.Fatalf("unexpected error (guard must not surface as an error): %v", err)
	}
	assertAdminEditUserOutcomeXOR(t, outcome)
	if outcome.Validation == nil || outcome.Validation.Field != "roleIds" {
		t.Fatalf("outcome.Validation = %+v, want field=roleIds", outcome.Validation)
	}
	// Proof the lookup ran through the tx-scoped, locking method: the stub was
	// reached exactly once, with the submitted role IDs, inside the single tx.
	if roles.findCalls != 1 {
		t.Fatalf("FindByIDsTx calls = %d, want 1 (in-tx FOR UPDATE lookup)", roles.findCalls)
	}
	assertStringSliceEqual(t, roles.lastFindIDs, []string{"r-admin", "r-missing"})
	if *txCalls != 1 {
		t.Fatalf("tx calls = %d, want 1 (lookup + guard run inside tx)", *txCalls)
	}
	if userRoles.setCalls != 0 {
		t.Fatalf("SetUserRolesTx calls = %d, want 0 (guard aborts before write)", userRoles.setCalls)
	}
}

// TestAdminUser_EditUser_RefetchUserDisappeared covers the unusual race
// between tx-commit and refetch: the row vanished after the mutation
// succeeded. The error chain preserves ErrNotFound for downstream checks.
func TestAdminUser_EditUser_RefetchUserDisappeared(t *testing.T) {
	t.Parallel()

	// UpdateTxVersioned sees the row, but FindByID after the tx returns
	// ErrNotFound to simulate a post-commit disappearance.
	users := &mockAdminUserRepository{
		users:   map[string]*domain.User{"u-target": {ID: "u-target"}},
		findErr: repository.ErrNotFound,
	}
	userRoles := &mockAdminUserRoleRepository{}
	authChk := &adminAuthChecker{admins: map[string]bool{"admin-1": true}}
	uc, _, _, _ := buildAdminUC(users, nil, userRoles, authChk)

	_, err := uc.EditUser(adminCallerCtx("admin-1"), "u-target", AdminEditUserInput{
		RoleIDs: []string{"r-general"},
	})
	// UpdateTxVersioned and SetUserRolesTx run in the tx and succeed. The
	// refetch then returns ErrNotFound, which is wrapped with the 'user
	// disappeared' message.
	assertInternalChain(t, err, "usecase: admin user edit: refetch: user disappeared")
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected repository.ErrNotFound in chain, got %T: %v", err, err)
	}
}

// TestAdminUser_EditUser_RefetchCancelled verifies that a cancelled context
// from refetchUser propagates as CANCELLED, not INTERNAL.
func TestAdminUser_EditUser_RefetchCancelled(t *testing.T) {
	t.Parallel()

	users := &mockAdminUserRepository{
		users:   map[string]*domain.User{"u-target": {ID: "u-target"}},
		findErr: context.Canceled,
	}
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

	users := &mockAdminUserRepository{
		users:   map[string]*domain.User{"u-target": {ID: "u-target"}},
		findErr: errors.New("db down"),
	}
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

// assertAdminEditUserOutcomeXOR asserts that exactly one of User, Validation,
// CannotRevokeOwnAdmin, or ConcurrentUpdate is set in the outcome.
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
	if outcome.ConcurrentUpdate {
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
