package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

// TestAdminCreateMasterCardgroup_SuccessVariant verifies a created master is
// mapped to the CreateMasterCardgroupSuccess union variant with cardCount 0.
func TestAdminCreateMasterCardgroup_SuccessVariant(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{createOut: usecase.CreateMasterOutcome{
		Master: &domain.MasterCardgroup{
			ID:      "m1",
			Name:    domain.CardgroupName("Deck"),
			Status:  domain.MasterStatusDraft,
			Version: 1,
		},
	}}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	got, err := mr.AdminCreateMasterCardgroup(context.Background(), model.CreateMasterCardgroupInput{Name: "Deck"})
	require.NoError(t, err)
	success, ok := got.(model.CreateMasterCardgroupSuccess)
	require.True(t, ok, "want CreateMasterCardgroupSuccess, got %T", got)
	require.NotNil(t, success.Master)
	assert.Equal(t, "m1", success.Master.ID)
	assert.Equal(t, 0, success.Master.CardCount)
	assert.Equal(t, model.MasterCardgroupStatusDraft, success.Master.Status)
}

// TestAdminCreateMasterCardgroup_ValidationVariant verifies a usecase validation
// outcome is mapped to the InputValidationError union variant as data (nil error).
func TestAdminCreateMasterCardgroup_ValidationVariant(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{createOut: usecase.CreateMasterOutcome{
		Validation: usecase.NewInputValidationInfo("name", "name is required"),
	}}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	got, err := mr.AdminCreateMasterCardgroup(context.Background(), model.CreateMasterCardgroupInput{Name: ""})
	require.NoError(t, err)
	ve, ok := got.(model.InputValidationError)
	require.True(t, ok, "want InputValidationError, got %T", got)
	assert.Equal(t, "name", ve.Field)
	assert.Equal(t, "name is required", ve.Message)
}

// TestAdminUpdateMasterCardgroup_SuccessVariant verifies an updated master is
// mapped to the UpdateMasterCardgroupSuccess union variant with the correct card count.
func TestAdminUpdateMasterCardgroup_SuccessVariant(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{updateOut: usecase.UpdateMasterOutcome{
		Master: &domain.MasterCardgroup{
			ID:      "m1",
			Name:    domain.CardgroupName("Deck"),
			Status:  domain.MasterStatusDraft,
			Version: 2,
		},
		CardCount: 7,
	}}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	name := "Deck"
	got, err := mr.AdminUpdateMasterCardgroup(context.Background(), "m1", model.UpdateMasterCardgroupInput{Name: &name})
	require.NoError(t, err)
	success, ok := got.(model.UpdateMasterCardgroupSuccess)
	require.True(t, ok, "want UpdateMasterCardgroupSuccess, got %T", got)
	require.NotNil(t, success.Master)
	assert.Equal(t, "m1", success.Master.ID)
	assert.Equal(t, 7, success.Master.CardCount)
}

// TestAdminUpdateMasterCardgroup_ValidationVariant verifies a usecase validation
// outcome is mapped to the InputValidationError union variant as data (nil error).
func TestAdminUpdateMasterCardgroup_ValidationVariant(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{updateOut: usecase.UpdateMasterOutcome{
		Validation: usecase.NewInputValidationInfo("name", "name is required"),
	}}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	got, err := mr.AdminUpdateMasterCardgroup(context.Background(), "m1", model.UpdateMasterCardgroupInput{})
	require.NoError(t, err)
	ve, ok := got.(model.InputValidationError)
	require.True(t, ok, "want InputValidationError, got %T", got)
	assert.Equal(t, "name", ve.Field)
	assert.Equal(t, "name is required", ve.Message)
}

// TestAdminCreateMasterCardgroup_WrapsForbidden verifies a usecase ForbiddenError
// is wrapped via gqlerr.FromUsecaseError into a FORBIDDEN-coded wire error.
func TestAdminCreateMasterCardgroup_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{createErr: ucerr.NewForbiddenError("admin only")}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	_, err := mr.AdminCreateMasterCardgroup(context.Background(), model.CreateMasterCardgroupInput{Name: "Deck"})
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}

// TestAdminPublishMasterCardgroup_EmptyVariant verifies a zero-card publish
// outcome is mapped to the MasterCardgroupEmptyError union variant.
func TestAdminPublishMasterCardgroup_EmptyVariant(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{publishOut: usecase.PublishMasterOutcome{EmptyMaster: true}}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	got, err := mr.AdminPublishMasterCardgroup(context.Background(), "m1")
	require.NoError(t, err)
	_, ok := got.(model.MasterCardgroupEmptyError)
	require.True(t, ok, "want MasterCardgroupEmptyError, got %T", got)
}

// TestAdminPublishMasterCardgroup_SuccessVariant verifies a published master is
// mapped to the PublishMasterCardgroupSuccess union variant with its card count.
func TestAdminPublishMasterCardgroup_SuccessVariant(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{publishOut: usecase.PublishMasterOutcome{
		Master: &domain.MasterCardgroup{
			ID:      "m1",
			Name:    domain.CardgroupName("Deck"),
			Status:  domain.MasterStatusPublished,
			Version: 2,
		},
		CardCount: 3,
	}}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	got, err := mr.AdminPublishMasterCardgroup(context.Background(), "m1")
	require.NoError(t, err)
	success, ok := got.(model.PublishMasterCardgroupSuccess)
	require.True(t, ok, "want PublishMasterCardgroupSuccess, got %T", got)
	require.NotNil(t, success.Master)
	assert.Equal(t, model.MasterCardgroupStatusPublished, success.Master.Status)
	assert.Equal(t, 3, success.Master.CardCount)
}

// TestAdminUnpublishMasterCardgroup_Success verifies the bare-object resolver
// maps the MasterWithCount carrier to *model.MasterCardgroup.
func TestAdminUnpublishMasterCardgroup_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{unpublishRes: &usecase.MasterWithCount{
		Master: &domain.MasterCardgroup{
			ID:      "m1",
			Name:    domain.CardgroupName("Deck"),
			Status:  domain.MasterStatusDraft,
			Version: 2,
		},
		CardCount: 4,
	}}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	got, err := mr.AdminUnpublishMasterCardgroup(context.Background(), "m1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "m1", got.ID)
	assert.Equal(t, model.MasterCardgroupStatusDraft, got.Status)
	assert.Equal(t, 4, got.CardCount)
}

// TestAdminUnpublishMasterCardgroup_WrapsValidation verifies a usecase error from
// the bare-object unpublish path is wrapped into a BAD_USER_INPUT wire error.
func TestAdminUnpublishMasterCardgroup_WrapsValidation(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{unpublishErr: ucerr.NewValidationError("id", "master cardgroup not found")}
	mr := &mutationResolver{&Resolver{MasterCatalogUC: stub}}

	_, err := mr.AdminUnpublishMasterCardgroup(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeBadUserInput), "want BAD_USER_INPUT wire code")
}

// TestAdminDeleteMasterCardgroup_Success verifies the scalar Boolean! resolver
// returns true on a nil usecase error.
func TestAdminDeleteMasterCardgroup_Success(t *testing.T) {
	t.Parallel()
	mr := &mutationResolver{&Resolver{MasterCatalogUC: &stubMasterCatalogUC{}}}

	ok, err := mr.AdminDeleteMasterCardgroup(context.Background(), "m1")
	require.NoError(t, err)
	assert.True(t, ok)
}

// TestAdminDeleteMasterCardgroup_WrapsForbidden verifies the delete resolver
// wraps a usecase ForbiddenError into a FORBIDDEN-coded wire error.
func TestAdminDeleteMasterCardgroup_WrapsForbidden(t *testing.T) {
	t.Parallel()
	mr := &mutationResolver{&Resolver{MasterCatalogUC: &stubMasterCatalogUC{deleteErr: ucerr.NewForbiddenError("admin only")}}}

	_, err := mr.AdminDeleteMasterCardgroup(context.Background(), "m1")
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}

// TestAdminMasters_Success verifies the admin list resolver maps the usecase
// output to the wire connection.
func TestAdminMasters_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{adminOut: &usecase.MasterCatalogConnectionOutput{
		Items: []*usecase.MasterCatalogItem{
			{Cardgroup: &domain.MasterCardgroup{ID: "m1", Name: domain.CardgroupName("Draft"), Status: domain.MasterStatusDraft}, CardCount: 0},
		},
		TotalCount: 1,
		StartCur:   "m1",
		EndCur:     "m1",
	}}
	qr := &queryResolver{&Resolver{MasterCatalogUC: stub}}

	conn, err := qr.AdminMasters(context.Background(), nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, conn.Edges, 1)
	assert.Equal(t, "m1", conn.Edges[0].Node.ID)
	assert.Equal(t, 1, conn.TotalCount)
}

// TestAdminMasters_WrapsForbidden verifies the admin list resolver wraps a
// usecase ForbiddenError into a FORBIDDEN-coded wire error.
func TestAdminMasters_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{adminErr: ucerr.NewForbiddenError("admin only")}
	qr := &queryResolver{&Resolver{MasterCatalogUC: stub}}

	_, err := qr.AdminMasters(context.Background(), nil, nil, nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}
