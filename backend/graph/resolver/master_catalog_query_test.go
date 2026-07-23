package resolver

import (
	"context"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/gqlerr/gqlerrtest"
	"backend/internal/usecase"
	"backend/internal/usecase/ucerr"
)

// stubMasterCatalogUC records the input it received and returns canned results,
// so the resolver's input mapping and error wrapping can be unit-tested without
// a real usecase or database.
type stubMasterCatalogUC struct {
	gotInput usecase.MasterCatalogConnectionInput
	out      *usecase.MasterCatalogConnectionOutput
	err      error

	// Settable canned returns for the admin-management methods, exercised by
	// master_catalog_admin_resolver_test.go. Each defaults to its zero value so
	// the existing query tests (which never touch these) keep working.
	adminOut       *usecase.MasterCatalogConnectionOutput
	adminErr       error
	adminMaster    *usecase.MasterWithCount
	adminMasterErr error
	createOut      usecase.CreateMasterOutcome
	createErr      error
	updateOut      usecase.UpdateMasterOutcome
	updateErr      error
	publishOut     usecase.PublishMasterOutcome
	publishErr     error
	unpublishRes   *usecase.MasterWithCount
	unpublishErr   error
	deleteErr      error

	importOut usecase.ImportMasterOutcome
	importErr error
	gotImport string

	mergeOut usecase.MergeMasterOutcome
	mergeErr error

	previewOut usecase.PreviewMergeOutcome
	previewErr error

	seedOut []*domain.Cardgroup
	seedErr error

	gotFindPublishedID string
	findPublishedRes   *usecase.MasterWithCount
	findPublishedErr   error
}

func (s *stubMasterCatalogUC) ListPublishedConnection(
	_ context.Context, in usecase.MasterCatalogConnectionInput,
) (*usecase.MasterCatalogConnectionOutput, error) {
	s.gotInput = in
	return s.out, s.err
}

func (s *stubMasterCatalogUC) FindPublishedMaster(_ context.Context, id string) (*usecase.MasterWithCount, error) {
	s.gotFindPublishedID = id
	return s.findPublishedRes, s.findPublishedErr
}

func (s *stubMasterCatalogUC) ListAdminConnection(_ context.Context, _ usecase.MasterCatalogConnectionInput) (*usecase.MasterCatalogConnectionOutput, error) {
	return s.adminOut, s.adminErr
}

func (s *stubMasterCatalogUC) AdminMaster(_ context.Context, _ string) (*usecase.MasterWithCount, error) {
	return s.adminMaster, s.adminMasterErr
}

func (s *stubMasterCatalogUC) CreateMaster(_ context.Context, _ usecase.CreateMasterInput) (usecase.CreateMasterOutcome, error) {
	return s.createOut, s.createErr
}

func (s *stubMasterCatalogUC) UpdateMaster(_ context.Context, _ string, _ usecase.UpdateMasterInput) (usecase.UpdateMasterOutcome, error) {
	return s.updateOut, s.updateErr
}

func (s *stubMasterCatalogUC) PublishMaster(_ context.Context, _ string) (usecase.PublishMasterOutcome, error) {
	return s.publishOut, s.publishErr
}

func (s *stubMasterCatalogUC) UnpublishMaster(_ context.Context, _ string) (*usecase.MasterWithCount, error) {
	return s.unpublishRes, s.unpublishErr
}

func (s *stubMasterCatalogUC) DeleteMaster(_ context.Context, _ string) error { return s.deleteErr }

func (s *stubMasterCatalogUC) ImportMaster(_ context.Context, masterID string) (usecase.ImportMasterOutcome, error) {
	s.gotImport = masterID
	return s.importOut, s.importErr
}

func (s *stubMasterCatalogUC) SeedDefaultStarters(_ context.Context) ([]*domain.Cardgroup, error) {
	return s.seedOut, s.seedErr
}

func (s *stubMasterCatalogUC) MergeMaster(_ context.Context, _, _ string) (usecase.MergeMasterOutcome, error) {
	return s.mergeOut, s.mergeErr
}

func (s *stubMasterCatalogUC) PreviewMergeMaster(_ context.Context, _, _ string) (usecase.PreviewMergeOutcome, error) {
	return s.previewOut, s.previewErr
}

// TestQueryResolver_MasterCatalog_Success verifies the resolver maps the model
// enums to usecase enums on the way in and the usecase output to the wire
// connection on the way out.
func TestQueryResolver_MasterCatalog_Success(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{
		out: &usecase.MasterCatalogConnectionOutput{
			Items: []*usecase.MasterCatalogItem{
				{Cardgroup: &domain.MasterCardgroup{ID: "x", Name: domain.CardgroupName("X"), Status: domain.MasterStatusPublished}, CardCount: 5},
			},
			TotalCount: 1,
			StartCur:   "x",
			EndCur:     "x",
		},
	}
	qr := &queryResolver{&Resolver{MasterCatalogUC: stub}}

	first := 10
	orderBy := model.MasterCatalogOrderByName
	dir := model.SortOrderDesc
	conn, err := qr.MasterCatalog(context.Background(), &first, nil, nil, nil, nil, &orderBy, &dir)
	require.NoError(t, err)
	require.Len(t, conn.Edges, 1)
	assert.Equal(t, "x", conn.Edges[0].Node.ID)
	assert.Equal(t, 5, conn.Edges[0].Node.CardCount)
	assert.Equal(t, 1, conn.TotalCount)

	// The resolver casts the model enums to the usecase enums before the call.
	require.NotNil(t, stub.gotInput.First)
	assert.Equal(t, 10, *stub.gotInput.First)
	require.NotNil(t, stub.gotInput.OrderBy)
	assert.Equal(t, usecase.MasterCatalogOrderByName, *stub.gotInput.OrderBy)
	require.NotNil(t, stub.gotInput.OrderDirection)
	assert.Equal(t, usecase.SortOrderDesc, *stub.gotInput.OrderDirection)
}

// TestQueryResolver_MasterCatalog_WrapsUsecaseError verifies the mandatory
// gqlerr.FromUsecaseError wrap: typed usecase errors surface with the correct
// wire code rather than escaping uncoded.
func TestQueryResolver_MasterCatalog_WrapsUsecaseError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want gqlerr.Code
	}{
		{"unauthenticated", ucerr.ErrUnauthenticated, gqlerr.CodeUnauthenticated},
		{"validation", ucerr.NewValidationError("first", "bad"), gqlerr.CodeBadUserInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			qr := &queryResolver{&Resolver{MasterCatalogUC: &stubMasterCatalogUC{err: tc.err}}}
			_, err := qr.MasterCatalog(context.Background(), nil, nil, nil, nil, nil, nil, nil)
			require.Error(t, err)
			assert.True(t, gqlerrtest.IsCode(err, tc.want), "want wire code %s", tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// MasterCardgroup (public, catalog-visible single deck)
//
// The masterCardgroup resolver lives in master_card.resolvers.go but reads
// through MasterCatalogUC.FindPublishedMaster, so its tests live here alongside
// stubMasterCatalogUC.
// ---------------------------------------------------------------------------

// TestQueryResolver_MasterCardgroup_Success verifies the resolver maps the
// published deck to *model.MasterCardgroup, including the live cardCount the
// usecase hydrates.
func TestQueryResolver_MasterCardgroup_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{findPublishedRes: &usecase.MasterWithCount{
		Master: &domain.MasterCardgroup{
			ID:      "m1",
			Name:    domain.CardgroupName("Deck"),
			Status:  domain.MasterStatusPublished,
			Version: 1,
		},
		// Non-zero so a resolver that drops the hydrated count fails the assertion.
		CardCount: 7,
	}}
	qr := &queryResolver{&Resolver{MasterCatalogUC: stub}}

	got, err := qr.MasterCardgroup(context.Background(), "m1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "m1", got.ID)
	assert.Equal(t, model.MasterCardgroupStatusPublished, got.Status)
	assert.Equal(t, 7, got.CardCount)
	assert.Equal(t, "m1", stub.gotFindPublishedID)
}

// TestQueryResolver_MasterCardgroup_NotFoundReturnsNil verifies an unknown or
// DRAFT id (usecase returns nil, nil) surfaces as GraphQL null with no error —
// the non-disclosure gate must not leak draft existence as an error.
func TestQueryResolver_MasterCardgroup_NotFoundReturnsNil(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCatalogUC{} // findPublishedRes nil, findPublishedErr nil
	qr := &queryResolver{&Resolver{MasterCatalogUC: stub}}

	got, err := qr.MasterCardgroup(context.Background(), "missing")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestQueryResolver_MasterCardgroup_WrapsUsecaseError verifies the mandatory
// gqlerr.FromUsecaseError wrap for the error paths FindPublishedMaster can return.
func TestQueryResolver_MasterCardgroup_WrapsUsecaseError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		want gqlerr.Code
	}{
		{"unauthenticated", ucerr.ErrUnauthenticated, gqlerr.CodeUnauthenticated},
		{"internal", eris.New("usecase: db: connection reset"), gqlerr.CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			qr := &queryResolver{&Resolver{MasterCatalogUC: &stubMasterCatalogUC{findPublishedErr: tc.err}}}
			_, err := qr.MasterCardgroup(context.Background(), "m1")
			require.Error(t, err)
			assert.True(t, gqlerrtest.IsCode(err, tc.want), "want wire code %s", tc.want)
		})
	}
}
