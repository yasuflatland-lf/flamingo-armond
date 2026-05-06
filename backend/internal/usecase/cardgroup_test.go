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

// FindPageByOwner / CountByOwner are stubbed for interface conformance only;
// pagination behaviour is exercised by the usecase-level tests that supply
// their own mocks.
func (m *mockCardgroupRepository) FindPageByOwner(
	_ context.Context,
	_ string,
	_, _ *repository.CardgroupCursor,
	_, _ int,
	_ repository.CardgroupOrderBy,
	_ repository.SortOrder,
	_ *string,
) ([]*domain.Cardgroup, error) {
	return nil, nil
}

func (m *mockCardgroupRepository) CountByOwner(_ context.Context, _ string, _ *string) (int64, error) {
	return 0, nil
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
