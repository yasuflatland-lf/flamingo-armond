package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

// mockCardgroupRepository is a manual test double for CardgroupRepository.
type mockCardgroupRepository struct {
	// FindByID
	findResult *domain.Cardgroup
	findErr    error

	// FindByOwner
	findByOwnerResult []*domain.Cardgroup
	findByOwnerErr    error

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
	findPageResult []*domain.Cardgroup
	findPageErr    error
	countResult    int64
	countErr       error
}

func (m *mockCardgroupRepository) FindByID(_ context.Context, _ string) (*domain.Cardgroup, error) {
	return m.findResult, m.findErr
}

func (m *mockCardgroupRepository) FindByOwner(_ context.Context, _ string) ([]*domain.Cardgroup, error) {
	return m.findByOwnerResult, m.findByOwnerErr
}

func (m *mockCardgroupRepository) Create(_ context.Context, cg *domain.Cardgroup) error {
	m.capturedCreate = cg
	return m.createErr
}

func (m *mockCardgroupRepository) Update(_ context.Context, _ string, patch repository.CardgroupUpdate) (*domain.Cardgroup, error) {
	m.updateCalled = true
	m.capturedPatch = patch
	return m.updateResult, m.updateErr
}

func (m *mockCardgroupRepository) Delete(_ context.Context, _ string) error {
	m.deleteCalled = true
	return m.deleteErr
}

// FindPageByOwner returns findPageResult/findPageErr when set; otherwise nil.
func (m *mockCardgroupRepository) FindPageByOwner(
	_ context.Context,
	_ string,
	_, _ *repository.CardgroupCursor,
	_, _ int,
	_ repository.CardgroupOrderBy,
	_ repository.SortOrder,
	_ *string,
) ([]*domain.Cardgroup, error) {
	return m.findPageResult, m.findPageErr
}

// CountByOwner returns countResult/countErr when set; otherwise 0, nil.
func (m *mockCardgroupRepository) CountByOwner(_ context.Context, _ string, _ *string) (int64, error) {
	return m.countResult, m.countErr
}

// --- helpers already defined in user_test.go (authedCtx, anonCtx, ptr, assertGQLErr) ---

// cgAuthedCtx is a convenience wrapper for cardgroup tests.
func cgAuthedCtx(sub string) context.Context {
	return auth.ContextWithUser(context.Background(), &auth.AuthUser{Sub: sub})
}

// --- MyCardgroups tests ---

func TestCardgroupUsecase_MyCardgroups_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.MyCardgroups(anonCtx())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardgroupUsecase_MyCardgroups_AuthenticatedReturnsRepoResult(t *testing.T) {
	t.Parallel()
	cgs := []*domain.Cardgroup{
		{ID: "cg1", OwnerID: "user-1", Name: "Alpha"},
		{ID: "cg2", OwnerID: "user-1", Name: "Beta"},
	}
	repo := &mockCardgroupRepository{findByOwnerResult: cgs}
	uc := NewCardgroupUsecase(repo)

	got, err := uc.MyCardgroups(cgAuthedCtx("user-1"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 cardgroups, got %d", len(got))
	}
}

func TestCardgroupUsecase_MyCardgroups_RepoError(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{findByOwnerErr: errors.New("db died")}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.MyCardgroups(cgAuthedCtx("user-1"))

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "INTERNAL", "")
}

// --- Cardgroup tests ---

func TestCardgroupUsecase_Cardgroup_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.Cardgroup(anonCtx(), "cg1")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardgroupUsecase_Cardgroup_NotFound_ReturnsNilNoError(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{findErr: repository.ErrNotFound}
	uc := NewCardgroupUsecase(repo)

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
	cg := &domain.Cardgroup{ID: "cg1", OwnerID: "user-1", Name: "Mine"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo)

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
	cg := &domain.Cardgroup{ID: "cg1", OwnerID: "user-1", Name: "Mine"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.Cardgroup(cgAuthedCtx("user-2"), "cg1")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !gqlerr.IsCode(err, gqlerr.CodeUnauthenticated) {
		t.Fatalf("expected UNAUTHENTICATED, got: %v", err)
	}
}

// --- Create tests ---

func TestCardgroupUsecase_Create_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.Create(anonCtx(), CreateCardgroupInput{Name: "Hello"})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardgroupUsecase_Create_EmptyName_BadUserInput(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: ""})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "BAD_USER_INPUT", "name")
}

func TestCardgroupUsecase_Create_TooLong_BadUserInput(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: strings.Repeat("a", 101)})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "BAD_USER_INPUT", "name")
}

func TestCardgroupUsecase_Create_Trims(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	got, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "  Hello  "})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Hello" {
		t.Fatalf("expected trimmed name %q, got %q", "Hello", got.Name)
	}
	if repo.capturedCreate == nil || repo.capturedCreate.Name != "Hello" {
		t.Fatal("expected repo.Create to receive trimmed name")
	}
}

func TestCardgroupUsecase_Create_Success_AssignsOwnerToCaller(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	got, err := uc.Create(cgAuthedCtx("user-1"), CreateCardgroupInput{Name: "My Group"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.OwnerID != "user-1" {
		t.Fatalf("expected OwnerID=%q, got %q", "user-1", got.OwnerID)
	}
	if got.ID == "" {
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

// --- Update tests ---

func TestCardgroupUsecase_Update_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.Update(anonCtx(), "cg1", UpdateCardgroupInput{Name: ptr("New")})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardgroupUsecase_Update_NonOwner_Unauthenticated(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{ID: "cg1", OwnerID: "user-1", Name: "Original"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.Update(cgAuthedCtx("user-2"), "cg1", UpdateCardgroupInput{Name: ptr("Hacked")})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardgroupUsecase_Update_NotFound_Unauthenticated(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{findErr: repository.ErrNotFound}
	uc := NewCardgroupUsecase(repo)

	_, err := uc.Update(cgAuthedCtx("user-1"), "cg-missing", UpdateCardgroupInput{Name: ptr("Anything")})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardgroupUsecase_Update_NameChange_Success(t *testing.T) {
	t.Parallel()
	existing := &domain.Cardgroup{ID: "cg1", OwnerID: "user-1", Name: "Old"}
	updated := &domain.Cardgroup{ID: "cg1", OwnerID: "user-1", Name: "New"}
	repo := &mockCardgroupRepository{findResult: existing, updateResult: updated}
	uc := NewCardgroupUsecase(repo)

	got, err := uc.Update(cgAuthedCtx("user-1"), "cg1", UpdateCardgroupInput{Name: ptr("New")})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "New" {
		t.Fatalf("expected name %q, got %q", "New", got.Name)
	}
	if !repo.updateCalled {
		t.Fatal("expected repo.Update to be called")
	}
	if repo.capturedPatch.Name == nil || *repo.capturedPatch.Name != "New" {
		t.Fatalf("expected patch.Name=%q, got %v", "New", repo.capturedPatch.Name)
	}
}

func TestCardgroupUsecase_Update_EmptyPatch_NoWrite(t *testing.T) {
	t.Parallel()
	existing := &domain.Cardgroup{ID: "cg1", OwnerID: "user-1", Name: "Original"}
	repo := &mockCardgroupRepository{findResult: existing}
	uc := NewCardgroupUsecase(repo)

	got, err := uc.Update(cgAuthedCtx("user-1"), "cg1", UpdateCardgroupInput{Name: nil})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Name != "Original" {
		t.Fatalf("expected existing row returned, got %+v", got)
	}
	if repo.updateCalled {
		t.Fatal("expected repo.Update NOT to be called for empty patch")
	}
}

// --- Delete tests ---

func TestCardgroupUsecase_Delete_Anonymous(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	err := uc.Delete(anonCtx(), "cg1")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardgroupUsecase_Delete_NonOwner_Unauthenticated(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{ID: "cg1", OwnerID: "user-1", Name: "Mine"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo)

	err := uc.Delete(cgAuthedCtx("user-2"), "cg1")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
	if repo.deleteCalled {
		t.Fatal("expected repo.Delete NOT to be called for non-owner")
	}
}

func TestCardgroupUsecase_Delete_NotFound_Unauthenticated(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{findErr: repository.ErrNotFound}
	uc := NewCardgroupUsecase(repo)

	err := uc.Delete(cgAuthedCtx("user-1"), "cg-missing")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	assertGQLErr(t, err, "UNAUTHENTICATED", "")
}

func TestCardgroupUsecase_Delete_Success(t *testing.T) {
	t.Parallel()
	cg := &domain.Cardgroup{ID: "cg1", OwnerID: "user-1", Name: "Mine"}
	repo := &mockCardgroupRepository{findResult: cg}
	uc := NewCardgroupUsecase(repo)

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
	uc := NewCardgroupUsecase(repo)

	after := "cursor-a"
	before := "cursor-b"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		After:  &after,
		Before: &before,
	})

	assertGQLErr(t, err, "BAD_USER_INPUT", "after")
}

// TestCardgroupUC_ConnectionGuards_FirstAndBefore verifies that combining first
// (forward page size) with before (backward cursor) is rejected.
func TestCardgroupUC_ConnectionGuards_FirstAndBefore(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	first := 5
	before := "cursor-b"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		First:  &first,
		Before: &before,
	})

	assertGQLErr(t, err, "BAD_USER_INPUT", "before")
}

// TestCardgroupUC_ConnectionGuards_LastAndAfter verifies that combining last
// (backward page size) with after (forward cursor) is rejected.
func TestCardgroupUC_ConnectionGuards_LastAndAfter(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	last := 5
	after := "cursor-a"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		Last:  &last,
		After: &after,
	})

	assertGQLErr(t, err, "BAD_USER_INPUT", "after")
}

// TestCardgroupUC_ConnectionGuards_BeforeAlone verifies that before without
// a companion last value is rejected as ambiguous.
func TestCardgroupUC_ConnectionGuards_BeforeAlone(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	before := "cursor-b"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		Before: &before,
	})

	assertGQLErr(t, err, "BAD_USER_INPUT", "before")
}

// TestCardgroupUC_ConnectionGuards_AfterAlone verifies that after without a
// companion first value is rejected as ambiguous.
func TestCardgroupUC_ConnectionGuards_AfterAlone(t *testing.T) {
	t.Parallel()
	repo := &mockCardgroupRepository{}
	uc := NewCardgroupUsecase(repo)

	after := "cursor-a"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("user-1"), CardgroupConnectionInput{
		After: &after,
	})

	assertGQLErr(t, err, "BAD_USER_INPUT", "after")
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
	foreignCG := &domain.Cardgroup{ID: "cg-owner-a", OwnerID: "owner-a", Name: "Foreign"}
	repo := &mockCardgroupRepository{findResult: foreignCG}
	uc := NewCardgroupUsecase(repo)

	first := 5
	after := "cg-owner-a"
	_, err := uc.ListCardgroupsByOwnerConnection(cgAuthedCtx("owner-b"), CardgroupConnectionInput{
		First: &first,
		After: &after,
	})

	assertGQLErr(t, err, "BAD_USER_INPUT", "after")
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
	// The mock returns all 3 rows (simulating repo returning first+1=3 rows).
	repo := &mockCardgroupRepository{
		findPageResult: cgs,
		countResult:    3,
	}
	uc := NewCardgroupUsecase(repo)

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
	matched := []*domain.Cardgroup{
		{ID: "cg1", OwnerID: "user-1", Name: "apple"},
	}
	repo := &mockCardgroupRepository{
		findPageResult: matched,
		countResult:    1,
	}
	uc := NewCardgroupUsecase(repo)

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
