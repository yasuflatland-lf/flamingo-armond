package usecase

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/require"

	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase/ucerr"
)

// newTestAdminGate builds an AdminGate backed by a mockAdminChecker (defined in
// helpers_test.go). Tests that exercise admin-gated methods use this to control
// whether the caller is treated as an admin.
func newTestAdminGate(isAdmin bool) *AdminGate {
	return NewAdminGate(&mockAdminChecker{isAdmin: isAdmin})
}

// masterCardgroup returns a minimal *domain.MasterCardgroup for test assertions.
func masterCardgroup(id string) *domain.MasterCardgroup {
	now := time.Now().UTC()
	return &domain.MasterCardgroup{
		ID:        id,
		Name:      domain.CardgroupName("Deck " + id),
		Status:    domain.MasterStatusDraft,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ---------------------------------------------------------------------------
// CreateMaster
// ---------------------------------------------------------------------------

func TestMasterCatalog_CreateMaster_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(false), newTestLogger())
	ctx := authedCtx("u1")
	_, err := uc.CreateMaster(ctx, CreateMasterInput{Name: "Deck"})
	var forbidden *ucerr.ForbiddenError
	require.ErrorAs(t, err, &forbidden)
}

func TestMasterCatalog_CreateMaster_InvalidNameValidation(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	out, err := uc.CreateMaster(ctx, CreateMasterInput{Name: "   "}) // empty after trim
	require.NoError(t, err)
	require.Nil(t, out.Master)
	require.NotNil(t, out.Validation)
	require.Equal(t, "name", out.Validation.Field)
}

func TestMasterCatalog_CreateMaster_InvalidDescriptionValidation(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	tooLong := strings.Repeat("a", domain.DescriptionMax+1)
	out, err := uc.CreateMaster(ctx, CreateMasterInput{Name: "My Deck", Description: &tooLong})
	require.NoError(t, err)
	require.Nil(t, out.Master)
	require.NotNil(t, out.Validation)
	require.Equal(t, "description", out.Validation.Field)
}

func TestMasterCatalog_CreateMaster_Success(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	out, err := uc.CreateMaster(ctx, CreateMasterInput{Name: "My Deck"})
	require.NoError(t, err)
	require.Nil(t, out.Validation)
	require.NotNil(t, out.Master)
	require.Equal(t, domain.MasterStatusDraft, out.Master.Status)
	require.Equal(t, 1, out.Master.Version)
	require.Len(t, repo.createCalls, 1)
}

// ---------------------------------------------------------------------------
// UpdateMaster
// ---------------------------------------------------------------------------

func TestMasterCatalog_UpdateMaster_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(false), newTestLogger())
	ctx := authedCtx("u1")
	_, err := uc.UpdateMaster(ctx, "some-id", UpdateMasterInput{})
	var forbidden *ucerr.ForbiddenError
	require.ErrorAs(t, err, &forbidden)
}

func TestMasterCatalog_UpdateMaster_InvalidName(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	name := "   "
	out, err := uc.UpdateMaster(ctx, "some-id", UpdateMasterInput{Name: &name})
	require.NoError(t, err)
	require.Nil(t, out.Master)
	require.NotNil(t, out.Validation)
	require.Equal(t, "name", out.Validation.Field)
}

func TestMasterCatalog_UpdateMaster_InvalidDescription(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	tooLong := strings.Repeat("a", domain.DescriptionMax+1)
	out, err := uc.UpdateMaster(ctx, "some-id", UpdateMasterInput{Description: &tooLong})
	require.NoError(t, err)
	require.Nil(t, out.Master)
	require.NotNil(t, out.Validation)
	require.Equal(t, "description", out.Validation.Field)
}

func TestMasterCatalog_UpdateMaster_Success(t *testing.T) {
	t.Parallel()
	mcg := masterCardgroup("id-1")
	repo := &mockMasterCatalogRepository{
		updateFn: func(_ string, _ repository.MasterCardgroupUpdate) (*domain.MasterCardgroup, error) {
			return mcg, nil
		},
		countCardsRes: 5,
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	newName := "Updated"
	out, err := uc.UpdateMaster(ctx, "id-1", UpdateMasterInput{Name: &newName})
	require.NoError(t, err)
	require.Nil(t, out.Validation)
	require.NotNil(t, out.Master)
	require.Equal(t, int64(5), out.CardCount)
}

func TestMasterCatalog_UpdateMaster_NotFound(t *testing.T) {
	t.Parallel()
	// updateFn defaults to returning ErrNotFound when nil in mock.
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	out, err := uc.UpdateMaster(ctx, "missing", UpdateMasterInput{})
	require.NoError(t, err)
	require.Nil(t, out.Master)
	require.NotNil(t, out.Validation)
	require.Equal(t, "id", out.Validation.Field)
}

// ---------------------------------------------------------------------------
// PublishMaster
// ---------------------------------------------------------------------------

func TestMasterCatalog_PublishMaster_EmptyMasterRejected(t *testing.T) {
	t.Parallel()
	mcg := masterCardgroup("id-1")
	var publishCalled bool
	repo := &mockMasterCatalogRepository{
		findByIDFn:    func(_ string) (*domain.MasterCardgroup, error) { return mcg, nil },
		countCardsRes: 0,
		publishFn:     func(_ string) (*domain.MasterCardgroup, error) { publishCalled = true; return mcg, nil },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	out, err := uc.PublishMaster(ctx, "id-1")
	require.NoError(t, err)
	require.True(t, out.EmptyMaster)
	require.Nil(t, out.Master)
	require.False(t, publishCalled, "publishFn must NOT be called for empty decks")
}

func TestMasterCatalog_PublishMaster_Success(t *testing.T) {
	t.Parallel()
	mcg := masterCardgroup("id-1")
	published := *mcg
	published.Status = domain.MasterStatusPublished
	published.Version = 2
	repo := &mockMasterCatalogRepository{
		findByIDFn:    func(_ string) (*domain.MasterCardgroup, error) { return mcg, nil },
		countCardsRes: 3,
		publishFn:     func(_ string) (*domain.MasterCardgroup, error) { return &published, nil },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	out, err := uc.PublishMaster(ctx, "id-1")
	require.NoError(t, err)
	require.False(t, out.EmptyMaster)
	require.NotNil(t, out.Master)
	require.Equal(t, domain.MasterStatusPublished, out.Master.Status)
	require.Equal(t, int64(3), out.CardCount)
}

// ---------------------------------------------------------------------------
// UnpublishMaster
// ---------------------------------------------------------------------------

func TestMasterCatalog_UnpublishMaster_Success(t *testing.T) {
	t.Parallel()
	mcg := masterCardgroup("id-1")
	mcg.Status = domain.MasterStatusPublished
	draft := *mcg
	draft.Status = domain.MasterStatusDraft
	repo := &mockMasterCatalogRepository{
		unpublishFn:   func(_ string) (*domain.MasterCardgroup, error) { return &draft, nil },
		countCardsRes: 7,
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	result, err := uc.UnpublishMaster(ctx, "id-1")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, domain.MasterStatusDraft, result.Master.Status)
	require.Equal(t, int64(7), result.CardCount)
}

func TestMasterCatalog_UnpublishMaster_NotFound(t *testing.T) {
	t.Parallel()
	// unpublishFn defaults to ErrNotFound when nil in mock.
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	_, err := uc.UnpublishMaster(ctx, "missing")
	// lowerValidationInfo converts the not-found info into a *ucerr.ValidationError.
	var ve *ucerr.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, "id", ve.Field)
}

// ---------------------------------------------------------------------------
// DeleteMaster
// ---------------------------------------------------------------------------

func TestMasterCatalog_DeleteMaster_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(false), newTestLogger())
	ctx := authedCtx("u1")
	err := uc.DeleteMaster(ctx, "some-id")
	var forbidden *ucerr.ForbiddenError
	require.ErrorAs(t, err, &forbidden)
}

func TestMasterCatalog_DeleteMaster_Success(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{deleteErr: nil}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	err := uc.DeleteMaster(ctx, "some-id")
	require.NoError(t, err)
}

func TestMasterCatalog_DeleteMaster_NotFound(t *testing.T) {
	t.Parallel()
	repo := &mockMasterCatalogRepository{deleteErr: repository.ErrNotFound}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	err := uc.DeleteMaster(ctx, "some-id")
	var ve *ucerr.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, "id", ve.Field)
}

// ---------------------------------------------------------------------------
// ListAdminConnection
// ---------------------------------------------------------------------------

func TestMasterCatalog_ListAdminConnection_NonAdminForbidden(t *testing.T) {
	t.Parallel()
	uc := NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, newTestAdminGate(false), newTestLogger())
	ctx := authedCtx("u1")
	_, err := uc.ListAdminConnection(ctx, MasterCatalogConnectionInput{})
	var forbidden *ucerr.ForbiddenError
	require.ErrorAs(t, err, &forbidden)
}

func TestMasterCatalog_ListAdminConnection_ReturnsPage(t *testing.T) {
	t.Parallel()
	items := []*repository.MasterCatalogItem{
		{Cardgroup: masterCardgroup("a"), CardCount: 2},
		{Cardgroup: masterCardgroup("b"), CardCount: 0},
	}
	repo := &mockMasterCatalogRepository{
		findPageAnyStatusTotal: 2,
		findPageAnyStatus:      items,
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	out, err := uc.ListAdminConnection(ctx, MasterCatalogConnectionInput{
		First: intPtr(10),
	})
	require.NoError(t, err)
	require.Equal(t, int64(2), out.TotalCount)
	require.Len(t, out.Items, 2)
}

// ---------------------------------------------------------------------------
// NilAdminGate constructor panic
// ---------------------------------------------------------------------------

func TestNewMasterCatalogUsecase_NilAdminGatePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil adminGate")
		}
	}()
	NewMasterCatalogUsecase(&mockMasterCatalogRepository{}, &mockCopyMasterToUserUC{}, nil, newTestLogger())
}

// ---------------------------------------------------------------------------
// PublishMaster not-found
// ---------------------------------------------------------------------------

func TestMasterCatalog_PublishMaster_NotFound(t *testing.T) {
	t.Parallel()
	var publishCalled bool
	repo := &mockMasterCatalogRepository{
		findByIDFn: func(_ string) (*domain.MasterCardgroup, error) {
			return nil, repository.ErrNotFound
		},
		countCardsRes: 3,
		publishFn: func(_ string) (*domain.MasterCardgroup, error) {
			publishCalled = true
			return nil, nil
		},
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	_, err := uc.PublishMaster(ctx, "missing-id")
	require.Error(t, err)
	var ve *ucerr.ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, "id", ve.Field)
	require.False(t, publishCalled, "Publish must NOT be called when the deck is not found")
}

// ---------------------------------------------------------------------------
// UpdateMaster CountCards error after successful Update
// ---------------------------------------------------------------------------

func TestMasterCatalog_UpdateMaster_CountCardsErrorAfterUpdate(t *testing.T) {
	t.Parallel()
	mcg := masterCardgroup("id-1")
	sentinel := eris.New("count cards failure")
	repo := &mockMasterCatalogRepository{
		updateFn: func(_ string, _ repository.MasterCardgroupUpdate) (*domain.MasterCardgroup, error) {
			return mcg, nil
		},
		countCardsErr: sentinel,
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	newName := "New name"
	out, err := uc.UpdateMaster(ctx, "id-1", UpdateMasterInput{Name: &newName})
	require.Error(t, err)
	require.True(t, errors.Is(err, sentinel))
	require.Zero(t, out.CardCount)
	require.Nil(t, out.Master)
}

// ---------------------------------------------------------------------------
// PublishMaster CountCards error (deck exists, count fails)
// ---------------------------------------------------------------------------

func TestMasterCatalog_PublishMaster_CountCardsError(t *testing.T) {
	t.Parallel()
	mcg := masterCardgroup("id-1")
	sentinel := eris.New("count boom")
	var publishCalled bool
	repo := &mockMasterCatalogRepository{
		findByIDFn:    func(_ string) (*domain.MasterCardgroup, error) { return mcg, nil },
		countCardsErr: sentinel,
		publishFn:     func(_ string) (*domain.MasterCardgroup, error) { publishCalled = true; return mcg, nil },
	}
	uc := NewMasterCatalogUsecase(repo, &mockCopyMasterToUserUC{}, newTestAdminGate(true), newTestLogger())
	ctx := authedCtx("admin1")
	out, err := uc.PublishMaster(ctx, "id-1")
	require.Error(t, err)
	require.True(t, errors.Is(err, sentinel))
	require.False(t, out.EmptyMaster, "count error must not be misclassified as empty deck")
	require.False(t, publishCalled, "Publish must NOT be called when CountCards fails")
}
