package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/repository"
)

// mockCardgroupRepository is a manual test double for CardgroupRepository.
type mockCardgroupRepository struct {
	// FindByID
	findResult *domain.Cardgroup
	findErr    error

	// findByIDFn, when non-nil, takes precedence over (findResult, findErr)
	// and lets a test return different rows for different cursor IDs (used
	// by the cross-orderBy cursor-hydration tests).
	findByIDFn func(id string) (*domain.Cardgroup, error)

	// lastFoundCardgroup is set to the aggregate returned by the most recent
	// successful FindByID call. Update tests use this to verify that the
	// aggregate's Name was mutated by Rename before repo.Update is called.
	lastFoundCardgroup *domain.Cardgroup

	// nameAtUpdateCall snapshots lastFoundCardgroup.Name at the moment Update is
	// invoked. Rename mutates the aggregate referenced by lastFoundCardgroup, so a
	// post-hoc read always sees the post-Rename value; capturing here proves the
	// mutation preceded the repository call.
	nameAtUpdateCall domain.CardgroupName

	// Create
	createErr      error
	capturedCreate *domain.Cardgroup

	// Update
	updateResult  *domain.Cardgroup
	updateErr     error
	capturedPatch repository.CardgroupUpdate
	updateCalled  bool

	// Delete
	deleteErr    error
	deleteCalled bool

	// FindPageByOwner / CountByOwner — used by pagination tests.
	// findPageTotal is the filtered totalCount FindPageByOwner now returns
	// alongside the page rows (the connection path reads its total from here,
	// not from CountByOwner). countResult/countErr remain for the filter-less
	// CountByOwner callers (the cardgroup-limit check on Create).
	findPageResult []*domain.Cardgroup
	findPageTotal  int64
	findPageErr    error
	countResult    int64
	countErr       error

	// Captured arguments from the last FindPageByOwner / CountByOwner calls.
	// These let pagination tests assert that the +1 fetch trick, the orderBy
	// translation, and the cursor hydration reach the repository correctly.
	findPageCalls []findPageByOwnerCall
	countCalls    []countByOwnerCall
}

type findPageByOwnerCall struct {
	OwnerID string
	After   *repository.CardgroupCursor
	Before  *repository.CardgroupCursor
	First   int
	Last    int
	OrderBy repository.CardgroupOrderBy
	Dir     repository.SortOrder
	Search  *string
}

type countByOwnerCall struct {
	OwnerID string
	Search  *string
}

func (m *mockCardgroupRepository) FindByID(_ context.Context, id string) (*domain.Cardgroup, error) {
	if m.findByIDFn != nil {
		cg, err := m.findByIDFn(id)
		if err == nil {
			m.lastFoundCardgroup = cg
		}
		return cg, err
	}
	if m.findErr == nil {
		m.lastFoundCardgroup = m.findResult
	}
	return m.findResult, m.findErr
}

func (m *mockCardgroupRepository) Create(_ context.Context, cg *domain.Cardgroup) error {
	m.capturedCreate = cg
	return m.createErr
}

func (m *mockCardgroupRepository) Update(_ context.Context, _ string, patch repository.CardgroupUpdate) (*domain.Cardgroup, error) {
	if m.lastFoundCardgroup != nil {
		m.nameAtUpdateCall = m.lastFoundCardgroup.Name
	}
	m.updateCalled = true
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}

func (m *mockCardgroupRepository) Delete(_ context.Context, _ string) error {
	m.deleteCalled = true
	return m.deleteErr
}

// FindPageByOwner returns findPageResult/findPageTotal/findPageErr when set;
// otherwise nil/0/nil. Each call is captured in findPageCalls so tests can
// assert on the arguments the usecase forwarded (e.g. first+1, orderBy
// translation, cursor hydration).
func (m *mockCardgroupRepository) FindPageByOwner(
	_ context.Context,
	ownerID string,
	after, before *repository.CardgroupCursor,
	first, last int,
	orderBy repository.CardgroupOrderBy,
	dir repository.SortOrder,
	search *string,
) ([]*domain.Cardgroup, int64, error) {
	m.findPageCalls = append(m.findPageCalls, findPageByOwnerCall{
		OwnerID: ownerID,
		After:   after,
		Before:  before,
		First:   first,
		Last:    last,
		OrderBy: orderBy,
		Dir:     dir,
		Search:  search,
	})
	return m.findPageResult, m.findPageTotal, m.findPageErr
}

// CountByOwner returns countResult/countErr when set; otherwise 0, nil.
func (m *mockCardgroupRepository) CountByOwner(_ context.Context, ownerID string, search *string) (int64, error) {
	m.countCalls = append(m.countCalls, countByOwnerCall{OwnerID: ownerID, Search: search})
	return m.countResult, m.countErr
}

// --- helpers (authedCtx, anonCtx, ptr) are defined in user_test.go;
// assertion helpers (assertUnauthenticated, assertValidationError,
// assertForbidden, assertCancelled, assertInternalChain) live in
// helpers_test.go. ---

// cgAuthedCtx is a convenience wrapper for cardgroup tests.
func cgAuthedCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
}

// cgDefaultAdmin returns a fresh admin stub that reports isAdmin=true. Every
// existing Create/Update/Find/pagination test passes this so checkCardgroupLimit
// short-circuits before CountByOwner is reached — the prior behaviour of those
// tests is unchanged. The per-call-site freshness keeps the stub's call counter
// isolated. mockAdminChecker is defined in helpers_test.go.
func cgDefaultAdmin() *mockAdminChecker {
	return &mockAdminChecker{isAdmin: true}
}

// --- Cardgroup tests ---

func TestCardgroupUsecase_Cardgroup_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	_, err := uc.Cardgroup(anonCtx(), "cg1")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
}

func TestCardgroupUsecase_Cardgroup_NotFound_ReturnsNilNoError(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{findErr: repository.ErrNotFound}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	got, err := uc.Cardgroup(cgAuthedCtx("user-1"), "cg-missing")

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil cardgroup, got: %+v", got)
	}
}

func TestCardgroupUsecase_Cardgroup_OwnerSeesOwn(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Mine"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	got, err := uc.Cardgroup(cgAuthedCtx("user-1"), "cg1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.ID != "cg1" {
		t.Fatalf("expected cardgroup cg1, got %+v", got)
	}
}

func TestCardgroupUsecase_Cardgroup_NonOwnerGetsUnauthenticated(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Mine"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	_, err := uc.Cardgroup(cgAuthedCtx("user-2"), "cg1")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
}

// --- Create tests ---

func TestCardgroupUsecase_Create_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	_, err := uc.Create(anonCtx(), CreateCardgroupInput{Name: "Hello"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
}

func TestCardgroupUsecase_Create_EmptyName_ValidationVariant(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: ""})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Cardgroup != nil {
		t.Fatal("expected nil Cardgroup on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on empty name")
	}
	if outcome.Validation.Field != "name" {
		t.Fatalf("expected Validation.Field=%q, got %q", "name", outcome.Validation.Field)
	}
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called on validation failure")
	}
}

func TestCardgroupUsecase_Create_TooLong_ValidationVariant(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: strings.Repeat("a", 101)})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Cardgroup != nil {
		t.Fatal("expected nil Cardgroup on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on too-long name")
	}
	if outcome.Validation.Field != "name" {
		t.Fatalf("expected Validation.Field=%q, got %q", "name", outcome.Validation.Field)
	}
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called on validation failure")
	}
}

func TestCardgroupUsecase_Create_Trims(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "  Hello  "})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Cardgroup == nil {
		t.Fatal("expected non-nil Cardgroup on success")
	}
	if outcome.Cardgroup.Name != "Hello" {
		t.Fatalf("expected trimmed name %q, got %q", "Hello", outcome.Cardgroup.Name)
	}
	if repo.capturedCreate == nil || repo.capturedCreate.Name != "Hello" {
		t.Fatal("expected repo.Create to receive trimmed name")
	}
}

func TestCardgroupUsecase_Create_Success_AssignsOwnerToCaller(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "My Group"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Cardgroup == nil {
		t.Fatal("expected non-nil Cardgroup on success")
	}
	if outcome.Cardgroup.OwnerID != "user-1" {
		t.Fatalf("expected OwnerID=%q, got %q", "user-1", outcome.Cardgroup.OwnerID)
	}
	if outcome.Cardgroup.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if repo.capturedCreate == nil {
		t.Fatal("expected repo.Create to be called")
	}
	if repo.capturedCreate.OwnerID != "user-1" {
		t.Fatalf("expected repo.Create called with OwnerID=%q, got %q", "user-1", repo.capturedCreate.OwnerID)
	}
	if repo.capturedCreate.ID == "" {
		t.Fatal("expected repo.Create called with non-empty ID")
	}
}

// --- Create per-user cardgroup-limit tests ---

// TestCardgroupUsecase_Create_GeneralUser_UnderLimit_Succeeds verifies that a
// non-admin owner holding 4 cardgroups (below the limit of 5) can create one
// more: the outcome carries the new Cardgroup and LimitReached stays nil.
func TestCardgroupUsecase_Create_GeneralUser_UnderLimit_Succeeds(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{countResult: 4}
	uc := NewCardgroupUsecase(repo, &mockAdminChecker{isAdmin: false}, newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "Fifth"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Cardgroup == nil {
		t.Fatal("expected non-nil Cardgroup when under the limit")
	}
	if outcome.LimitReached != nil {
		t.Fatalf("expected nil LimitReached under the limit, got %+v", outcome.LimitReached)
	}
	if repo.capturedCreate == nil {
		t.Fatal("expected repo.Create to be called when under the limit")
	}
}

// TestCardgroupUsecase_Create_GeneralUser_AtLimit_Rejected verifies that a
// non-admin owner already holding 5 cardgroups (the limit) is rejected via the
// LimitReached outcome, and repo.Create is NOT called.
func TestCardgroupUsecase_Create_GeneralUser_AtLimit_Rejected(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{countResult: domain.GeneralUserCardgroupLimit}
	uc := NewCardgroupUsecase(repo, &mockAdminChecker{isAdmin: false}, newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "Sixth"})

	if err != nil {
		t.Fatalf("expected nil error (limit goes to outcome), got: %v", err)
	}
	if outcome.Cardgroup != nil {
		t.Fatalf("expected nil Cardgroup at the limit, got %+v", outcome.Cardgroup)
	}
	if outcome.LimitReached == nil {
		t.Fatal("expected non-nil LimitReached at the limit")
	}
	if outcome.LimitReached.Limit != domain.GeneralUserCardgroupLimit {
		t.Fatalf("expected LimitReached.Limit=%d, got %d", domain.GeneralUserCardgroupLimit, outcome.LimitReached.Limit)
	}
	if outcome.LimitReached.Current != domain.GeneralUserCardgroupLimit {
		t.Fatalf("expected LimitReached.Current=%d, got %d", domain.GeneralUserCardgroupLimit, outcome.LimitReached.Current)
	}
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called when the limit is reached")
	}
}

// TestCardgroupUsecase_Create_GeneralUser_AboveLimit_Rejected pins the
// "existing 6+ users: reject-new-only" behaviour: an owner already holding 6
// cardgroups (above the cap of 5) is rejected and LimitReached.Current reports
// the real count rather than the cap.
func TestCardgroupUsecase_Create_GeneralUser_AboveLimit_Rejected(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{countResult: 6}
	uc := NewCardgroupUsecase(repo, &mockAdminChecker{isAdmin: false}, newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "Seventh"})

	if err != nil {
		t.Fatalf("expected nil error (limit goes to outcome), got: %v", err)
	}
	if outcome.LimitReached == nil {
		t.Fatal("expected non-nil LimitReached above the limit")
	}
	if outcome.LimitReached.Current != 6 {
		t.Fatalf("expected LimitReached.Current=6 (real count, not the cap), got %d", outcome.LimitReached.Current)
	}
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called when above the limit")
	}
}

// TestCardgroupUsecase_Create_Admin_SkipsCounting verifies that an admin owner
// bypasses the cardgroup limit entirely: repo.Create runs even though the
// owner's count would be at the cap, and CountByOwner is NEVER called because
// the admin short-circuit precedes the count query.
func TestCardgroupUsecase_Create_Admin_SkipsCounting(t *testing.T) {
	t.Parallel()
	// countResult is set to the cap to prove that even when the count WOULD
	// reject a general user, an admin is never subjected to it.
	repo := &mockCardgroupRepository{countResult: domain.GeneralUserCardgroupLimit}
	admin := &mockAdminChecker{isAdmin: true}
	uc := NewCardgroupUsecase(repo, admin, newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("admin-1"), CreateCardgroupInput{Name: "AdminGroup"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Cardgroup == nil {
		t.Fatal("expected non-nil Cardgroup for admin owner")
	}
	if outcome.LimitReached != nil {
		t.Fatalf("expected nil LimitReached for admin owner, got %+v", outcome.LimitReached)
	}
	if repo.capturedCreate == nil {
		t.Fatal("expected repo.Create to be called for admin owner")
	}
	if len(repo.countCalls) != 0 {
		t.Fatalf("expected CountByOwner NOT to be called for admin owner, got %d calls", len(repo.countCalls))
	}
}

// TestCardgroupUsecase_Create_IsAdminError_Wrapped verifies that a non-context
// error from IsAdmin propagates wrapped with the "usecase: cardgroup: check
// admin" prefix and that repo.Create is not reached.
func TestCardgroupUsecase_Create_IsAdminError_Wrapped(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	admin := &mockAdminChecker{err: errors.New("auth: lookup failed")}
	uc := NewCardgroupUsecase(repo, admin, newTestLogger())

	_, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "Group"})

	if err == nil {
		t.Fatal("expected error from IsAdmin, got nil")
	}
	assertInternalChain(t, err, "usecase: cardgroup: check admin")
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called when IsAdmin fails")
	}
}

// TestCardgroupUsecase_Create_CountByOwnerError_Wrapped verifies that a
// non-context error from CountByOwner propagates wrapped with the "usecase:
// cardgroup: count by owner" prefix.
func TestCardgroupUsecase_Create_CountByOwnerError_Wrapped(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{countErr: errors.New("db: count failed")}
	uc := NewCardgroupUsecase(repo, &mockAdminChecker{isAdmin: false}, newTestLogger())

	_, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "Group"})

	if err == nil {
		t.Fatal("expected error from CountByOwner, got nil")
	}
	assertInternalChain(t, err, "usecase: cardgroup: count by owner")
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called when CountByOwner fails")
	}
}

// TestCardgroupUsecase_Create_IsAdminCancelled_IdentityPreserved verifies that
// a context.Canceled returned from IsAdmin is propagated with its identity
// intact (errors.Is matches) and is NOT wrapped — the resolver matches the
// cancellation via errors.Is, which an eris wrap would defeat.
func TestCardgroupUsecase_Create_IsAdminCancelled_IdentityPreserved(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	admin := &mockAdminChecker{err: context.Canceled}
	uc := NewCardgroupUsecase(repo, admin, newTestLogger())

	_, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "Group"})

	if err == nil {
		t.Fatal("expected context.Canceled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected errors.Is(err, context.Canceled)=true, got %v (%T)", err, err)
	}
	// Identity preserved means the returned error IS context.Canceled, not an
	// eris wrap around it.
	if err != context.Canceled {
		t.Fatalf("expected the unwrapped context.Canceled, got %v (%T)", err, err)
	}
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called when IsAdmin is cancelled")
	}
}

// TestCardgroupUsecase_Create_LimitAndInvalidName_NameValidationFirst pins the
// ordering: name validation runs BEFORE the limit check. With an invalid name
// and a count that would otherwise trip the limit, the outcome carries
// Validation (not LimitReached), and neither IsAdmin nor CountByOwner is
// consulted.
func TestCardgroupUsecase_Create_LimitAndInvalidName_NameValidationFirst(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{countResult: domain.GeneralUserCardgroupLimit}
	admin := &mockAdminChecker{isAdmin: false}
	uc := NewCardgroupUsecase(repo, admin, newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: ""})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation when the name is invalid")
	}
	if outcome.Validation.Field != "name" {
		t.Fatalf("expected Validation.Field=%q, got %q", "name", outcome.Validation.Field)
	}
	if outcome.LimitReached != nil {
		t.Fatalf("expected nil LimitReached when name validation wins, got %+v", outcome.LimitReached)
	}
	if outcome.Cardgroup != nil {
		t.Fatal("expected nil Cardgroup on validation failure")
	}
	if admin.calls != 0 {
		t.Fatalf("expected IsAdmin NOT to be called when name validation fails first, got %d calls", admin.calls)
	}
	if len(repo.countCalls) != 0 {
		t.Fatalf("expected CountByOwner NOT to be called when name validation fails first, got %d calls", len(repo.countCalls))
	}
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called on validation failure")
	}
}

// --- Update tests ---

func TestCardgroupUsecase_Update_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	_, err := uc.Update(anonCtx(), "cg1", UpdateCardgroupInput{Name: ptr("New")})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
}

func TestCardgroupUsecase_Update_NonOwner_Unauthenticated(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Original"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	_, err := uc.Update(cgAuthedCtx("user-2"), "cg1", UpdateCardgroupInput{Name: ptr("Hacked")})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
}

func TestCardgroupUsecase_Update_NotFound_Unauthenticated(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{findErr: repository.ErrNotFound}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	_, err := uc.Update(cgAuthedCtx("user-1"), "cg-missing", UpdateCardgroupInput{Name: ptr("Anything")})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
}

func TestCardgroupUsecase_Update_NameChange_Success(t *testing.T) {
	t.Parallel()
	existing := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Old"}
	updated := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "New"}
	repo := &mockCardgroupRepository{findResult: existing, updateResult: updated}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Update(cgAuthedCtx("user-1"), "cg1", UpdateCardgroupInput{Name: ptr("New")})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Cardgroup == nil {
		t.Fatal("expected non-nil Cardgroup on success")
	}
	if outcome.Cardgroup.Name != "New" {
		t.Fatalf("expected name %q, got %q", "New", outcome.Cardgroup.Name)
	}
	if outcome.Validation != nil {
		t.Fatalf("expected nil Validation on success, got %+v", outcome.Validation)
	}
	if !repo.updateCalled {
		t.Fatal("expected repo.Update to be called")
	}
	if repo.capturedPatch.Name == nil || *repo.capturedPatch.Name != "New" {
		t.Fatalf("expected patch.Name=%q, got %v", "New", repo.capturedPatch.Name)
	}
	// nameAtUpdateCall snapshots aggregate.Name inside the mock's Update method
	// at call time. If Rename had been moved after repo.Update, this snapshot
	// would still hold the original name, so the assertion below catches both
	// "Rename removed" and "Rename after repo.Update" regressions.
	if repo.lastFoundCardgroup == nil {
		t.Fatal("expected FindByID to have been called and captured the aggregate")
	}
	if repo.nameAtUpdateCall != domain.CardgroupName("New") {
		t.Fatalf("aggregate.Name must be %q at the moment repo.Update is called (Rename must precede repo.Update), got %q",
			"New", repo.nameAtUpdateCall)
	}
}

func TestCardgroupUsecase_Update_EmptyPatch_NoWrite(t *testing.T) {
	t.Parallel()
	existing := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Original"}
	repo := &mockCardgroupRepository{findResult: existing}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Update(cgAuthedCtx("user-1"), "cg1", UpdateCardgroupInput{Name: nil})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if outcome.Cardgroup == nil || outcome.Cardgroup.Name != "Original" {
		t.Fatalf("expected existing row returned, got %+v", outcome)
	}
	if outcome.Validation != nil {
		t.Fatalf("expected nil Validation on empty patch, got %+v", outcome.Validation)
	}
	if repo.updateCalled {
		t.Fatal("expected repo.Update NOT to be called for empty patch")
	}
}

func TestCardgroupUsecase_Update_EmptyName_ValidationVariant(t *testing.T) {
	t.Parallel()
	existing := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Original"}
	repo := &mockCardgroupRepository{findResult: existing}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Update(cgAuthedCtx("user-1"), "cg1", UpdateCardgroupInput{Name: ptr("")})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Cardgroup != nil {
		t.Fatal("expected nil Cardgroup on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on empty name")
	}
	if outcome.Validation.Field != "name" {
		t.Fatalf("expected Validation.Field=%q, got %q", "name", outcome.Validation.Field)
	}
	if repo.updateCalled {
		t.Fatal("repository.Update must not be called on validation failure")
	}
}

func TestCardgroupUsecase_Update_TooLongName_ValidationVariant(t *testing.T) {
	t.Parallel()
	existing := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Original"}
	repo := &mockCardgroupRepository{findResult: existing}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	outcome, err := uc.Update(cgAuthedCtx("user-1"), "cg1", UpdateCardgroupInput{Name: ptr(strings.Repeat("a", 101))})

	if err != nil {
		t.Fatalf("expected nil error (validation goes to outcome), got: %v", err)
	}
	if outcome.Cardgroup != nil {
		t.Fatal("expected nil Cardgroup on validation failure")
	}
	if outcome.Validation == nil {
		t.Fatal("expected non-nil Validation on too-long name")
	}
	if outcome.Validation.Field != "name" {
		t.Fatalf("expected Validation.Field=%q, got %q", "name", outcome.Validation.Field)
	}
	if repo.updateCalled {
		t.Fatal("repository.Update must not be called on validation failure")
	}
}

func TestCardgroupUsecase_Update_RepoError_InfraChannel(t *testing.T) {
	t.Parallel()
	existing := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Original"}
	repo := &mockCardgroupRepository{
		findResult: existing,
		updateErr:  errors.New("db: connection lost"),
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	_, err := uc.Update(cgAuthedCtx("user-1"), "cg1", UpdateCardgroupInput{Name: ptr("New")})

	if err == nil {
		t.Fatal("expected error from repo, got nil")
	}
	assertInternalChain(t, err, "usecase: cardgroup: update")
}

// --- Delete tests ---

func TestCardgroupUsecase_Delete_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	err := uc.Delete(anonCtx(), "cg1")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
}

func TestCardgroupUsecase_Delete_NonOwner_Unauthenticated(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Mine"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	err := uc.Delete(cgAuthedCtx("user-2"), "cg1")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
	if repo.deleteCalled {
		t.Fatal("expected repo.Delete NOT to be called for non-owner")
	}
}

func TestCardgroupUsecase_Delete_NotFound_Unauthenticated(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{findErr: repository.ErrNotFound}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	err := uc.Delete(cgAuthedCtx("user-1"), "cg-missing")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertUnauthenticated(t, err)
}

func TestCardgroupUsecase_Delete_Success(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), OwnerID: "user-1", Name: "Mine"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	err := uc.Delete(cgAuthedCtx("user-1"), "cg1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.deleteCalled {
		t.Fatal("expected repo.Delete to be called")
	}
}

// ---------------------------------------------------------------------------
// ListCardgroupsByOwnerConnection — mixed-direction guard tests
// ---------------------------------------------------------------------------

// TestCardgroupUC_ConnectionGuards_AfterAndBefore verifies that supplying both
// after and before is rejected with BAD_USER_INPUT before any repo call.
func TestCardgroupUC_ConnectionGuards_AfterAndBefore(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	after := "cursor-a"
	before := "cursor-b"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		After:  &after,
		Before: &before,
	})

	assertValidationError(t, err, "after", "")
}

// TestCardgroupUC_ConnectionGuards_FirstAndBefore verifies that combining first
// (forward page size) with before (backward cursor) is rejected.
func TestCardgroupUC_ConnectionGuards_FirstAndBefore(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	before := "cursor-b"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		First:  &first,
		Before: &before,
	})

	assertValidationError(t, err, "before", "")
}

// TestCardgroupUC_ConnectionGuards_LastAndAfter verifies that combining last
// (backward page size) with after (forward cursor) is rejected.
func TestCardgroupUC_ConnectionGuards_LastAndAfter(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	last := 5
	after := "cursor-a"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		Last:  &last,
		After: &after,
	})

	assertValidationError(t, err, "after", "")
}

// TestCardgroupUC_ConnectionGuards_BeforeAlone verifies that before without
// a companion last value is rejected as ambiguous.
func TestCardgroupUC_ConnectionGuards_BeforeAlone(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	before := "cursor-b"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		Before: &before,
	})

	assertValidationError(t, err, "before", "")
}

// TestCardgroupUC_ConnectionGuards_AfterAlone verifies that after without a
// companion first value is rejected as ambiguous.
func TestCardgroupUC_ConnectionGuards_AfterAlone(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	after := "cursor-a"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		After: &after,
	})

	assertValidationError(t, err, "after", "")
}

// ---------------------------------------------------------------------------
// ListCardgroupsByOwnerConnection — cross-tenant cursor test
// ---------------------------------------------------------------------------

// TestCardgroupUC_Connection_CursorFromOtherOwner_BadUserInput verifies that
// passing a cursor (cardgroup ID) belonging to a different owner is rejected
// with BAD_USER_INPUT rather than leaking the existence of foreign cardgroups.
func TestCardgroupUC_Connection_CursorFromOtherOwner_BadUserInput(t *testing.T) {
	t.Parallel()

	// The cursor ID maps to a cardgroup owned by owner-A, not owner-B.
	foreignCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-owner-a"), OwnerID: "owner-a", Name: "Foreign"}
	repo := &mockCardgroupRepository{findResult: foreignCG}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-owner-a"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("owner-b"), CardgroupConnectionInput{
		First: &first,
		After: &after,
	})

	assertValidationError(t, err, "after", "")
}

// ---------------------------------------------------------------------------
// ListCardgroupsByOwnerConnection — happy-path tests
// ---------------------------------------------------------------------------

// TestCardgroupUC_Connection_FirstPage verifies the basic forward page-1
// scenario: 3 cardgroups in the repo, first=2 → 2 edges returned, hasNextPage
// true, totalCount 3.
func TestCardgroupUC_Connection_FirstPage(t *testing.T) {
	t.Parallel()

	cgs := []*domain.Cardgroup{
		{ID: "cg1", OwnerID: "user-1", Name: "Alpha"},
		{ID: "cg2", OwnerID: "user-1", Name: "Beta"},
		// The third row is the "+1" the usecase requests to detect hasNextPage.
		{ID: "cg3", OwnerID: "user-1", Name: "Gamma"},
	}
	// The mock returns all 3 rows (simulating repo returning first+1=3 rows)
	// plus the filtered total FindPageByOwner now carries.
	repo := &mockCardgroupRepository{
		findPageResult: cgs,
		findPageTotal:  3,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 2
	out, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		First: &first,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 3 {
		t.Fatalf("TotalCount = %d, want 3", out.TotalCount)
	}
	if len(out.Cardgroups) != 2 {
		t.Fatalf("len(Cardgroups) = %d, want 2 (trailing row trimmed)", len(out.Cardgroups))
	}
	if !out.HasNext {
		t.Fatal("HasNext = false, want true (overflow row detected)")
	}
	if out.HasPrev {
		t.Fatal("HasPrev = true, want false (no after cursor on page 1)")
	}
	if out.StartCur != "cg1" {
		t.Fatalf("StartCur = %q, want %q", out.StartCur, "cg1")
	}
	if out.EndCur != "cg2" {
		t.Fatalf("EndCur = %q, want %q", out.EndCur, "cg2")
	}
}

// TestCardgroupUC_Connection_Search verifies that a search filter is forwarded
// to the repository and the totalCount reflects the filtered count.
func TestCardgroupUC_Connection_Search(t *testing.T) {
	t.Parallel()

	// Only "apple" matches the search; "banana" is absent from the page result.
	// findPageTotal is the filtered count FindPageByOwner returns, so the
	// connection's totalCount reflects the search rather than the owner total.
	matched := []*domain.Cardgroup{
		{ID: "cg1", OwnerID: "user-1", Name: "apple"},
	}
	repo := &mockCardgroupRepository{
		findPageResult: matched,
		findPageTotal:  1,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 10
	search := "app"
	out, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		First:  &first,
		Search: &search,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.TotalCount != 1 {
		t.Fatalf("TotalCount = %d, want 1", out.TotalCount)
	}
	if len(out.Cardgroups) != 1 {
		t.Fatalf("len(Cardgroups) = %d, want 1", len(out.Cardgroups))
	}
	if out.Cardgroups[0].Name != "apple" {
		t.Fatalf("Cardgroups[0].Name = %q, want %q", out.Cardgroups[0].Name, "apple")
	}
}

// ---------------------------------------------------------------------------
// ListCardgroupsByOwnerConnection — additional coverage tests
// ---------------------------------------------------------------------------

// TestCardgroupUC_Connection_Anonymous verifies that an unauthenticated context
// short-circuits with UNAUTHENTICATED before any repository call.
func TestCardgroupUC_Connection_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 10
	_, err := uc.ListCardgroupsByOwnerConnection(anonCtx(), CardgroupConnectionInput{First: &first})
	assertUnauthenticated(t, err)
	if len(repo.findPageCalls) != 0 {
		t.Fatalf("expected no FindPageByOwner calls for anon ctx, got %d", len(repo.findPageCalls))
	}
}

// TestCardgroupUC_ConnectionGuards_FirstAndLast verifies that supplying both
// first and last is rejected with BAD_USER_INPUT(field="first") via
// resolveStandardPageSize.
func TestCardgroupUC_ConnectionGuards_FirstAndLast(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first, last := 5, 5
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
		Last:  &last,
	})
	assertValidationError(t, err, "first", "")
}

// TestCardgroupUC_Connection_FirstPage_AssertFirstPlusOne verifies that the
// usecase requests first+1 from the repository so the +1 fetch trick can
// detect hasNextPage. This locks down the guard against a future refactor
// that drops the increment.
func TestCardgroupUC_Connection_FirstPage_AssertFirstPlusOne(t *testing.T) {
	t.Parallel()

	cgs := []*domain.Cardgroup{
		{ID: "cg1", OwnerID: "u1", Name: "A"},
		{ID: "cg2", OwnerID: "u1", Name: "B"},
		{ID: "cg3", OwnerID: "u1", Name: "C"},
	}
	repo := &mockCardgroupRepository{findPageResult: cgs, countResult: 3}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 2
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.findPageCalls) != 1 {
		t.Fatalf("expected 1 FindPageByOwner call, got %d", len(repo.findPageCalls))
	}
	if repo.findPageCalls[0].First != 3 {
		t.Fatalf("expected repo.First=first+1=3, got %d", repo.findPageCalls[0].First)
	}
	if repo.findPageCalls[0].Last != 0 {
		t.Fatalf("expected repo.Last=0 on forward page, got %d", repo.findPageCalls[0].Last)
	}
}

// TestCardgroupUC_Connection_BackwardPagination_HappyPath verifies that
// last+before fetches last+1 rows and the leading overflow row is trimmed,
// HasPrev becomes true, and HasNext is true (because before != nil).
func TestCardgroupUC_Connection_BackwardPagination_HappyPath(t *testing.T) {
	t.Parallel()

	// Repo returns last+1=3 rows already reversed by the repository — the
	// usecase must drop the FIRST row (the overflow indicator), keeping the
	// final 2 as the page payload.
	cgs := []*domain.Cardgroup{
		{ID: "cg-overflow", OwnerID: "u1", Name: "Overflow"},
		{ID: "cg-a", OwnerID: "u1", Name: "Aplha"},
		{ID: "cg-b", OwnerID: "u1", Name: "Beta"},
	}
	now := time.Now().UTC()
	cursorCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-cursor"), OwnerID: "u1", Name: "Cursor", UpdatedAt: now}
	repo := &mockCardgroupRepository{
		findPageResult: cgs,
		countResult:    10,
		findResult:     cursorCG, // hydrate the before cursor
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	last := 2
	before := "cg-cursor"
	out, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		Last:   &last,
		Before: &before,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.Cardgroups) != 2 {
		t.Fatalf("expected 2 cardgroups (leading overflow trimmed), got %d", len(out.Cardgroups))
	}
	if out.Cardgroups[0].ID != "cg-a" {
		t.Fatalf("expected first=cg-a (overflow trimmed), got %s", out.Cardgroups[0].ID)
	}
	if !out.HasPrev {
		t.Fatal("expected HasPrev=true (overflow row detected)")
	}
	if !out.HasNext {
		t.Fatal("expected HasNext=true (before cursor present)")
	}
	if len(repo.findPageCalls) != 1 {
		t.Fatalf("expected 1 FindPageByOwner call, got %d", len(repo.findPageCalls))
	}
	if repo.findPageCalls[0].Last != 3 {
		t.Fatalf("expected repo.Last=last+1=3, got %d", repo.findPageCalls[0].Last)
	}
	if repo.findPageCalls[0].First != 0 {
		t.Fatalf("expected repo.First=0 on backward page, got %d", repo.findPageCalls[0].First)
	}
}

// TestCardgroupUC_Connection_OrderBy_Name_Asc verifies that orderBy=NAME is
// translated to repository.CardgroupOrderByName at the repo seam, and the
// direction is forwarded as SortAsc.
func TestCardgroupUC_Connection_OrderBy_Name_Asc(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	ob := CardgroupOrderByName
	dir := SortOrderAsc
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First:          &first,
		OrderBy:        &ob,
		OrderDirection: &dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.findPageCalls) != 1 {
		t.Fatalf("expected 1 repo call, got %d", len(repo.findPageCalls))
	}
	if repo.findPageCalls[0].OrderBy != repository.CardgroupOrderByName {
		t.Fatalf("expected repo.OrderBy=name, got %q", repo.findPageCalls[0].OrderBy)
	}
	if repo.findPageCalls[0].Dir != repository.SortAsc {
		t.Fatalf("expected repo.Dir=asc, got %q", repo.findPageCalls[0].Dir)
	}
}

// TestCardgroupUC_Connection_OrderBy_CreatedAt_Desc verifies the CREATED_AT/DESC
// branch of resolveCardgroupOrderBy.
func TestCardgroupUC_Connection_OrderBy_CreatedAt_Desc(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	ob := CardgroupOrderByCreatedAt
	dir := SortOrderDesc
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First:          &first,
		OrderBy:        &ob,
		OrderDirection: &dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.findPageCalls[0].OrderBy != repository.CardgroupOrderByCreatedAt {
		t.Fatalf("expected repo.OrderBy=created_at, got %q", repo.findPageCalls[0].OrderBy)
	}
	if repo.findPageCalls[0].Dir != repository.SortDesc {
		t.Fatalf("expected repo.Dir=desc, got %q", repo.findPageCalls[0].Dir)
	}
}

// TestCardgroupUC_Connection_OrderBy_ID_Asc verifies the ID/ASC branch of
// resolveCardgroupOrderBy.
func TestCardgroupUC_Connection_OrderBy_ID_Asc(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	ob := CardgroupOrderByID
	dir := SortOrderAsc
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First:          &first,
		OrderBy:        &ob,
		OrderDirection: &dir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.findPageCalls[0].OrderBy != repository.CardgroupOrderByID {
		t.Fatalf("expected repo.OrderBy=id, got %q", repo.findPageCalls[0].OrderBy)
	}
}

// TestCardgroupUC_Connection_DefaultOrderBy_WhenNil verifies that nil orderBy
// + nil orderDirection default to UPDATED_AT/DESC at the repository seam.
func TestCardgroupUC_Connection_DefaultOrderBy_WhenNil(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.findPageCalls[0].OrderBy != repository.CardgroupOrderByUpdatedAt {
		t.Fatalf("expected default repo.OrderBy=updated_at, got %q", repo.findPageCalls[0].OrderBy)
	}
	if repo.findPageCalls[0].Dir != repository.SortDesc {
		t.Fatalf("expected default repo.Dir=desc, got %q", repo.findPageCalls[0].Dir)
	}
}

// TestCardgroupUC_Connection_DefaultPageSize_WhenAllNil verifies that when
// neither first nor last is supplied (and neither is a cursor), the
// resolveStandardPageSize default of 20 is used.
func TestCardgroupUC_Connection_DefaultPageSize_WhenAllNil(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// defaultPageSize is 20; the +1 fetch trick brings it to 21.
	if repo.findPageCalls[0].First != defaultPageSize+1 {
		t.Fatalf("expected first=defaultPageSize+1=%d, got %d",
			defaultPageSize+1, repo.findPageCalls[0].First)
	}
}

// TestCardgroupUC_Connection_PageSize_Clamp verifies that an out-of-range
// first is clamped to maxPageSize before the +1 fetch is added.
func TestCardgroupUC_Connection_PageSize_Clamp(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 200 // above maxPageSize=100
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 200 -> clamped to 100 -> repo sees 100+1=101 (= pageCap).
	if repo.findPageCalls[0].First != maxPageSize+1 {
		t.Fatalf("expected first=maxPageSize+1=%d, got %d",
			maxPageSize+1, repo.findPageCalls[0].First)
	}
}

// TestCardgroupUC_Connection_PageSize_NegativeFirst verifies that a negative
// first is clamped to 0 (the +1 fetch trick is also skipped because wantFirst
// would have been 0).
func TestCardgroupUC_Connection_PageSize_NegativeFirst(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := -5
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.findPageCalls[0].First != 0 {
		t.Fatalf("expected first=0 (negative clamped), got %d", repo.findPageCalls[0].First)
	}
}

// TestCardgroupUC_Connection_CursorHydration_Name verifies that an `after`
// cursor under orderBy=NAME hydrates the Name column from the looked-up
// cardgroup, so the repository receives a cursor whose Name pointer is set.
func TestCardgroupUC_Connection_CursorHydration_Name(t *testing.T) {
	t.Parallel()

	cursorCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-cursor"), OwnerID: "u1", Name: "Mango"}
	repo := &mockCardgroupRepository{
		findResult:     cursorCG,
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-cursor"
	ob := CardgroupOrderByName
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First:   &first,
		After:   &after,
		OrderBy: &ob,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.findPageCalls[0].After
	if got == nil {
		t.Fatal("expected repo.After != nil")
	}
	if got.Name == nil {
		t.Fatal("expected hydrated Name pointer on cursor")
	}
	if *got.Name != "Mango" {
		t.Fatalf("expected hydrated Name=Mango, got %q", *got.Name)
	}
}

// TestCardgroupUC_Connection_CursorHydration_CreatedAt verifies cursor
// hydration for orderBy=CREATED_AT.
func TestCardgroupUC_Connection_CursorHydration_CreatedAt(t *testing.T) {
	t.Parallel()

	created := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	cursorCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-cursor"), OwnerID: "u1", Name: "X", CreatedAt: created}
	repo := &mockCardgroupRepository{
		findResult:     cursorCG,
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-cursor"
	ob := CardgroupOrderByCreatedAt
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First:   &first,
		After:   &after,
		OrderBy: &ob,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.findPageCalls[0].After
	if got == nil || got.CreatedAt == nil {
		t.Fatal("expected hydrated CreatedAt pointer on cursor")
	}
	if !got.CreatedAt.Equal(created) {
		t.Fatalf("expected CreatedAt=%v, got %v", created, *got.CreatedAt)
	}
}

// TestCardgroupUC_Connection_CursorHydration_UpdatedAt verifies cursor
// hydration for orderBy=UPDATED_AT (the default).
func TestCardgroupUC_Connection_CursorHydration_UpdatedAt(t *testing.T) {
	t.Parallel()

	updated := time.Date(2024, 6, 7, 8, 9, 10, 0, time.UTC)
	cursorCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-cursor"), OwnerID: "u1", Name: "X", UpdatedAt: updated}
	repo := &mockCardgroupRepository{
		findResult:     cursorCG,
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-cursor"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
		After: &after,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.findPageCalls[0].After
	if got == nil || got.UpdatedAt == nil {
		t.Fatal("expected hydrated UpdatedAt pointer on cursor")
	}
	if !got.UpdatedAt.Equal(updated) {
		t.Fatalf("expected UpdatedAt=%v, got %v", updated, *got.UpdatedAt)
	}
}

// TestCardgroupUC_Connection_CursorHydration_OrderByID verifies that when
// orderBy=ID, the cursor is still validated for ownership but no time/name
// column is hydrated (only ID is set).
func TestCardgroupUC_Connection_CursorHydration_OrderByID(t *testing.T) {
	t.Parallel()

	cursorCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-cursor"), OwnerID: "u1", Name: "X"}
	repo := &mockCardgroupRepository{
		findResult:     cursorCG,
		findPageResult: []*domain.Cardgroup{},
		countResult:    0,
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-cursor"
	ob := CardgroupOrderByID
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First:   &first,
		After:   &after,
		OrderBy: &ob,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := repo.findPageCalls[0].After
	if got == nil {
		t.Fatal("expected repo.After != nil for orderBy=ID cursor")
	}
	if got.ID != "cg-cursor" {
		t.Fatalf("expected cursor.ID=cg-cursor, got %q", got.ID)
	}
	if got.Name != nil || got.CreatedAt != nil || got.UpdatedAt != nil {
		t.Fatalf("expected only ID hydrated for orderBy=ID, got Name=%v CreatedAt=%v UpdatedAt=%v",
			got.Name, got.CreatedAt, got.UpdatedAt)
	}
}

// TestCardgroupUC_Connection_CursorHydration_OrderByID_NotFound verifies that
// a missing cursor under orderBy=ID surfaces BAD_USER_INPUT(field=after) so
// the caller cannot probe foreign cardgroup IDs.
func TestCardgroupUC_Connection_CursorHydration_OrderByID_NotFound(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{findErr: repository.ErrNotFound}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-missing"
	ob := CardgroupOrderByID
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First:   &first,
		After:   &after,
		OrderBy: &ob,
	})
	assertValidationError(t, err, "after", "")
}

// TestCardgroupUC_Connection_CursorHydration_OrderByID_RepoError verifies that
// a non-NotFound repo error during cursor lookup under orderBy=ID surfaces
// INTERNAL.
func TestCardgroupUC_Connection_CursorHydration_OrderByID_RepoError(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{findErr: errors.New("db died")}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-x"
	ob := CardgroupOrderByID
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First:   &first,
		After:   &after,
		OrderBy: &ob,
	})
	assertInternalChain(t, err, "usecase: cardgroup: hydrate cursor")
}

// TestCardgroupUC_Connection_CursorHydration_OrderByID_OtherOwner verifies that
// a cursor under orderBy=ID belonging to another owner is rejected as
// BAD_USER_INPUT(after) — the same posture as the cross-owner cursor test
// above but specifically through the orderBy=ID branch.
func TestCardgroupUC_Connection_CursorHydration_OrderByID_OtherOwner(t *testing.T) {
	t.Parallel()

	foreignCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-x"), OwnerID: "owner-a", Name: "Foreign"}
	repo := &mockCardgroupRepository{findResult: foreignCG}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-x"
	ob := CardgroupOrderByID
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("owner-b"), CardgroupConnectionInput{
		First:   &first,
		After:   &after,
		OrderBy: &ob,
	})
	assertValidationError(t, err, "after", "")
}

// TestCardgroupUC_Connection_CursorHydration_NotFound_BadUserInput verifies
// that a missing cursor under a non-ID orderBy surfaces BAD_USER_INPUT.
func TestCardgroupUC_Connection_CursorHydration_NotFound_BadUserInput(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{findErr: repository.ErrNotFound}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-missing"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
		After: &after,
	})
	assertValidationError(t, err, "after", "")
}

// TestCardgroupUC_Connection_CursorHydration_RepoError_Internal verifies that
// a non-NotFound repo error during cursor hydration surfaces INTERNAL.
func TestCardgroupUC_Connection_CursorHydration_RepoError_Internal(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{findErr: errors.New("db dead")}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cg-cursor"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
		After: &after,
	})
	assertInternalChain(t, err, "usecase: cardgroup: hydrate cursor")
}

// TestCardgroupUC_Connection_FindPageRepoError_Internal verifies that an
// unexpected repository error from FindPageByOwner surfaces INTERNAL.
func TestCardgroupUC_Connection_FindPageRepoError_Internal(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{
		findPageErr: errors.New("db dead"),
	}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("u1"), CardgroupConnectionInput{
		First: &first,
	})
	assertInternalChain(t, err, "usecase: cardgroup: find page by owner")
}

// ---------------------------------------------------------------------------
// resolveCardgroupOrderBy / resolveStandardPageSize / resolveCardgroupCursor
// — direct unit tests for branches that are hard to reach end-to-end.
// ---------------------------------------------------------------------------

// TestResolveCardgroupOrderBy_InvalidOrderBy hits the default arm of the
// switch on *orderBy by passing an enum value that isn't part of the
// allowlist. The function returns BAD_USER_INPUT.
func TestResolveCardgroupOrderBy_InvalidOrderBy(t *testing.T) {
	t.Parallel()

	bogus := CardgroupOrderBy("BOGUS")
	_, _, err := resolveCardgroupOrderBy(&bogus, nil)
	assertValidationError(t, err, "orderBy", "")
}

// TestResolveCardgroupOrderBy_InvalidDirection hits the default arm of the
// switch on *dir.
func TestResolveCardgroupOrderBy_InvalidDirection(t *testing.T) {
	t.Parallel()

	bogus := SortOrder("SIDEWAYS")
	_, _, err := resolveCardgroupOrderBy(nil, &bogus)
	assertValidationError(t, err, "orderDirection", "")
}

// TestResolveCardgroupPageSize_LastClamp verifies that the "last only" branch
// clamps an oversized last to maxPageSize.
func TestResolveCardgroupPageSize_LastClamp(t *testing.T) {
	t.Parallel()

	last := 9999
	first, gotLast, err := resolveStandardPageSize(nil, &last)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != 0 {
		t.Fatalf("expected first=0, got %d", first)
	}
	if gotLast != maxPageSize {
		t.Fatalf("expected last=%d (clamped), got %d", maxPageSize, gotLast)
	}
}

// TestResolveCardgroupPageSize_LastNegativeClampedToZero verifies that a
// negative last is clamped to 0 by the inner clamp function.
func TestResolveCardgroupPageSize_LastNegativeClampedToZero(t *testing.T) {
	t.Parallel()

	last := -7
	first, gotLast, err := resolveStandardPageSize(nil, &last)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first != 0 || gotLast != 0 {
		t.Fatalf("expected (0, 0) for negative last, got (%d, %d)", first, gotLast)
	}
}

// TestResolveCardgroupCursor_NilCursor verifies that a nil/empty cursorID
// short-circuits with (nil, nil) — the caller branch that means "no cursor".
func TestResolveCardgroupCursor_NilCursor(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	c, err := uc.(*cardgroupUsecase).resolveCardgroupCursor(
		context.Background(),
		nil, "u1", repository.CardgroupOrderByName, "after",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c != nil {
		t.Fatalf("expected nil cursor for nil ID, got %+v", c)
	}

	empty := ""
	c, err = uc.(*cardgroupUsecase).resolveCardgroupCursor(
		context.Background(),
		&empty, "u1", repository.CardgroupOrderByName, "after",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c != nil {
		t.Fatalf("expected nil cursor for empty ID, got %+v", c)
	}
}

// TestResolveCardgroupCursor_OtherOwner_NonIDOrderBy verifies the cross-tenant
// guard fires on a non-ID orderBy too.
func TestResolveCardgroupCursor_OtherOwner_NonIDOrderBy(t *testing.T) {
	t.Parallel()

	foreignCG := &domain.Cardgroup{ID: domain.CardgroupID("cg-x"), OwnerID: "owner-a", Name: "Foreign"}
	repo := &mockCardgroupRepository{findResult: foreignCG}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	id := "cg-x"
	_, err := uc.(*cardgroupUsecase).resolveCardgroupCursor(
		context.Background(),
		&id, "owner-b", repository.CardgroupOrderByName, "after",
	)
	assertValidationError(t, err, "after", "")
}

// TestResolveCardgroupCursor_UnknownOrderBy hits the impossible default arm
// of the switch in resolveCardgroupCursor, ensuring it returns INTERNAL
// rather than silently leaving the cursor unhydrated. This is defensive
// coverage for the documented "caller bug" branch.
func TestResolveCardgroupCursor_UnknownOrderBy(t *testing.T) {
	t.Parallel()

	cg := &domain.Cardgroup{ID: domain.CardgroupID("cg-cursor"), OwnerID: "u1", Name: "X"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	id := "cg-cursor"
	_, err := uc.(*cardgroupUsecase).resolveCardgroupCursor(
		context.Background(),
		&id, "u1", repository.CardgroupOrderBy("not_a_real_column"), "after",
	)
	assertInternalChain(t, err, "usecase: cardgroup: unhandled orderBy")
}

// TestTranslateCardgroupNameErr_DefaultArm verifies the default switch arm
// (an unexpected non-domain error) maps to INTERNAL.
func TestTranslateCardgroupNameErr_DefaultArm(t *testing.T) {
	t.Parallel()

	err := translateCardgroupNameErr(errors.New("surprise"))
	assertInternalChain(t, err, "usecase: translate cardgroup name error")
}

// TestResolveCardgroupCursor_MalformedV1_ReturnsBadUserInput verifies that a
// "v1:" envelope with an invalid base64 payload is rejected with BAD_USER_INPUT
// rather than INTERNAL. The cursor package marks the error as a decode failure;
// the usecase must map it to the correct GraphQL code.
func TestResolveCardgroupCursor_MalformedV1_ReturnsBadUserInput(t *testing.T) {
	t.Parallel()

	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	malformed := "v1:!!!not-base64!!!"
	_, err := uc.(*cardgroupUsecase).resolveCardgroupCursor(
		context.Background(),
		&malformed, "u1", repository.CardgroupOrderByID, "after",
	)
	assertValidationError(t, err, "after", "")
}

// TestResolveCardgroupCursor_V1EncodedID verifies backward-compat: a v1
// encoded cursor decodes to the raw ID and the DB lookup proceeds with that ID.
func TestResolveCardgroupCursor_V1EncodedID(t *testing.T) {
	t.Parallel()

	cg := &domain.Cardgroup{ID: domain.CardgroupID("cg-cursor"), OwnerID: "u1", Name: "X"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	// Encode the raw ID into the v1 envelope the way the resolver would.
	encoded := "v1:Y2ctY3Vyc29y" // base64.RawURLEncoding.EncodeToString([]byte("cg-cursor"))
	c, err := uc.(*cardgroupUsecase).resolveCardgroupCursor(
		context.Background(),
		&encoded, "u1", repository.CardgroupOrderByID, "after",
	)
	if err != nil {
		t.Fatalf("unexpected error for v1 encoded cursor: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cursor, got nil")
	}
	if c.ID != "cg-cursor" {
		t.Fatalf("expected decoded ID=cg-cursor, got %q", c.ID)
	}
}

// ---------------------------------------------------------------------------
// checkCardgroupLimit — direct unit tests for the shared limit helper.
// ---------------------------------------------------------------------------

// stubCardgroupCounter is a minimal cardgroupOwnerCounter for the direct
// checkCardgroupLimit tests. calls tracks invocations so admin-exempt and
// name-validation-first paths can assert the counter is never reached.
type stubCardgroupCounter struct {
	count int64
	err   error
	calls int
}

func (s *stubCardgroupCounter) CountByOwner(_ context.Context, _ string, _ *string) (int64, error) {
	s.calls++
	return s.count, s.err
}

// TestCheckCardgroupLimit_Admin_Exempt verifies that an admin owner is exempt:
// the helper returns (nil, nil) and the counter is never consulted.
func TestCheckCardgroupLimit_Admin_Exempt(t *testing.T) {
	t.Parallel()
	counter := &stubCardgroupCounter{count: domain.GeneralUserCardgroupLimit}
	admin := &mockAdminChecker{isAdmin: true}

	info, err := checkCardgroupLimit(context.Background(), counter, admin, "admin-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil LimitInfo for admin, got %+v", info)
	}
	if counter.calls != 0 {
		t.Fatalf("expected counter NOT to be called for admin, got %d calls", counter.calls)
	}
}

// TestCheckCardgroupLimit_UnderLimit verifies that a non-admin owner below the
// limit returns (nil, nil) — no limit info, no error.
func TestCheckCardgroupLimit_UnderLimit(t *testing.T) {
	t.Parallel()
	counter := &stubCardgroupCounter{count: domain.GeneralUserCardgroupLimit - 1}
	admin := &mockAdminChecker{isAdmin: false}

	info, err := checkCardgroupLimit(context.Background(), counter, admin, "user-1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info != nil {
		t.Fatalf("expected nil LimitInfo below the limit, got %+v", info)
	}
	if counter.calls != 1 {
		t.Fatalf("expected counter to be called once, got %d", counter.calls)
	}
}

// TestCheckCardgroupLimit_AtOrOverLimit verifies that a non-admin owner at or
// above the limit returns a non-nil CardgroupLimitInfo with the cap and the
// owner's real count.
func TestCheckCardgroupLimit_AtOrOverLimit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		count       int64
		wantCurrent int
	}{
		{name: "at limit", count: domain.GeneralUserCardgroupLimit, wantCurrent: domain.GeneralUserCardgroupLimit},
		{name: "over limit", count: domain.GeneralUserCardgroupLimit + 3, wantCurrent: domain.GeneralUserCardgroupLimit + 3},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			counter := &stubCardgroupCounter{count: tc.count}
			admin := &mockAdminChecker{isAdmin: false}

			info, err := checkCardgroupLimit(context.Background(), counter, admin, "user-1")

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if info == nil {
				t.Fatal("expected non-nil LimitInfo at/over the limit")
			}
			if info.Limit != domain.GeneralUserCardgroupLimit {
				t.Fatalf("expected Limit=%d, got %d", domain.GeneralUserCardgroupLimit, info.Limit)
			}
			if info.Current != tc.wantCurrent {
				t.Fatalf("expected Current=%d, got %d", tc.wantCurrent, info.Current)
			}
		})
	}
}

// TestCheckCardgroupLimit_IsAdminError_Wrapped verifies that a non-context
// IsAdmin error is wrapped with the "usecase: cardgroup: check admin" prefix
// and the counter is never consulted.
func TestCheckCardgroupLimit_IsAdminError_Wrapped(t *testing.T) {
	t.Parallel()
	counter := &stubCardgroupCounter{}
	admin := &mockAdminChecker{err: errors.New("auth: lookup failed")}

	_, err := checkCardgroupLimit(context.Background(), counter, admin, "user-1")

	assertInternalChain(t, err, "usecase: cardgroup: check admin")
	if counter.calls != 0 {
		t.Fatalf("expected counter NOT to be called when IsAdmin fails, got %d calls", counter.calls)
	}
}

// TestCheckCardgroupLimit_CountError_Wrapped verifies that a non-context
// CountByOwner error is wrapped with the "usecase: cardgroup: count by owner"
// prefix.
func TestCheckCardgroupLimit_CountError_Wrapped(t *testing.T) {
	t.Parallel()
	counter := &stubCardgroupCounter{err: errors.New("db: count failed")}
	admin := &mockAdminChecker{isAdmin: false}

	_, err := checkCardgroupLimit(context.Background(), counter, admin, "user-1")

	assertInternalChain(t, err, "usecase: cardgroup: count by owner")
}

// TestCheckCardgroupLimit_IsAdminCancelled_IdentityPreserved verifies that a
// context.Canceled from IsAdmin passes through unwrapped (errors.Is matches and
// the returned error IS context.Canceled), and the counter is never consulted.
func TestCheckCardgroupLimit_IsAdminCancelled_IdentityPreserved(t *testing.T) {
	t.Parallel()
	counter := &stubCardgroupCounter{}
	admin := &mockAdminChecker{err: context.Canceled}

	_, err := checkCardgroupLimit(context.Background(), counter, admin, "user-1")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected errors.Is(err, context.Canceled)=true, got %v (%T)", err, err)
	}
	if err != context.Canceled {
		t.Fatalf("expected the unwrapped context.Canceled, got %v (%T)", err, err)
	}
	if counter.calls != 0 {
		t.Fatalf("expected counter NOT to be called when IsAdmin is cancelled, got %d calls", counter.calls)
	}
}

// TestCardgroupUsecase_Create_CountByOwnerCancelled_IdentityPreserved verifies
// that a context.Canceled returned from CountByOwner propagates with its
// identity intact (errors.Is matches and err == context.Canceled, NOT an eris
// wrap) and that repo.Create is not reached. The admin stub reports isAdmin=false
// so the count query is reached and injects the cancellation.
func TestCardgroupUsecase_Create_CountByOwnerCancelled_IdentityPreserved(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{countErr: context.Canceled}
	uc := NewCardgroupUsecase(repo, &mockAdminChecker{isAdmin: false}, newTestLogger())

	outcome, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "Group"})

	if err == nil {
		t.Fatal("expected context.Canceled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected errors.Is(err, context.Canceled)=true, got %v (%T)", err, err)
	}
	// Identity preserved: the returned error IS context.Canceled, not an eris wrap.
	if err != context.Canceled {
		t.Fatalf("expected the unwrapped context.Canceled, got %v (%T)", err, err)
	}
	// The outcome must be the zero value — no LimitReached, no Cardgroup.
	if outcome.LimitReached != nil {
		t.Fatalf("expected nil LimitReached on cancellation, got %+v", outcome.LimitReached)
	}
	if outcome.Cardgroup != nil {
		t.Fatalf("expected nil Cardgroup on cancellation, got %+v", outcome.Cardgroup)
	}
	if repo.capturedCreate != nil {
		t.Fatal("repository.Create must not be called when CountByOwner is cancelled")
	}
}

// TestCheckCardgroupLimit_CountCancelled_IdentityPreserved verifies that a
// context.Canceled returned from CountByOwner passes through checkCardgroupLimit
// unwrapped: errors.Is matches and err == context.Canceled (bare identity).
func TestCheckCardgroupLimit_CountCancelled_IdentityPreserved(t *testing.T) {
	t.Parallel()
	counter := &stubCardgroupCounter{err: context.Canceled}
	admin := &mockAdminChecker{isAdmin: false}

	info, err := checkCardgroupLimit(context.Background(), counter, admin, "user-1")

	if err == nil {
		t.Fatal("expected context.Canceled, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected errors.Is(err, context.Canceled)=true, got %v (%T)", err, err)
	}
	// Identity preserved: the returned error IS context.Canceled, not an eris wrap.
	if err != context.Canceled {
		t.Fatalf("expected the unwrapped context.Canceled, got %v (%T)", err, err)
	}
	if info != nil {
		t.Fatalf("expected nil LimitInfo on cancellation, got %+v", info)
	}
}

// TestCheckCardgroupLimit_CountDeadlineExceeded_IdentityPreserved verifies that
// a context.DeadlineExceeded returned from CountByOwner passes through
// checkCardgroupLimit unwrapped: errors.Is matches and err ==
// context.DeadlineExceeded (bare identity). Mirrors the Canceled case above
// because isContextDone handles both sentinels.
func TestCheckCardgroupLimit_CountDeadlineExceeded_IdentityPreserved(t *testing.T) {
	t.Parallel()
	counter := &stubCardgroupCounter{err: context.DeadlineExceeded}
	admin := &mockAdminChecker{isAdmin: false}

	info, err := checkCardgroupLimit(context.Background(), counter, admin, "user-1")

	if err == nil {
		t.Fatal("expected context.DeadlineExceeded, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected errors.Is(err, context.DeadlineExceeded)=true, got %v (%T)", err, err)
	}
	// Identity preserved: the returned error IS context.DeadlineExceeded, not an eris wrap.
	if err != context.DeadlineExceeded {
		t.Fatalf("expected the unwrapped context.DeadlineExceeded, got %v (%T)", err, err)
	}
	if info != nil {
		t.Fatalf("expected nil LimitInfo on deadline exceeded, got %+v", info)
	}
}

// TestCardgroupUC_Connection_CursorFindByIDCancelled pins the cursor-hydration
// FindByID wrap site (resolveCardgroupCursor): a context.Canceled surfaced while
// hydrating the cursor must reach the caller unwrapped so the resolver routes it
// to Cancelled via errors.Is. assertCancelled alone is too weak — errors.Is walks
// the eris chain, so it passes even for eris.Wrap(context.Canceled, ...); the bare
// err == context.Canceled check pins that no wrap snuck in. See
// docs/backend/error-wrapping/pin-unwrapped-context-error-with-identity-check.md.
func TestCardgroupUC_Connection_CursorFindByIDCancelled(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{findErr: context.Canceled}
	uc := NewCardgroupUsecase(repo, cgDefaultAdmin(), newTestLogger())

	first := 5
	after := "cur-1"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		First: &first,
		After: &after,
	})

	assertCancelled(t, err)
	if err != context.Canceled {
		t.Fatalf("expected unwrapped context.Canceled, got %v", err)
	}
}
