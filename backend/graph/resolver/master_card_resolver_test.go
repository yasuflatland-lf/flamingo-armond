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

// stubMasterCardUC records the input it received and returns canned results, so
// the resolver's input mapping and error wrapping can be unit-tested without a
// real usecase or database.
type stubMasterCardUC struct {
	adminOut *usecase.MasterWithCount
	adminErr error

	gotListInput usecase.MasterCardConnectionInput
	listOut      *usecase.MasterCardConnectionOutput
	listErr      error
}

func (s *stubMasterCardUC) AdminMaster(_ context.Context, _ string) (*usecase.MasterWithCount, error) {
	return s.adminOut, s.adminErr
}

func (s *stubMasterCardUC) ListMasterCards(_ context.Context, in usecase.MasterCardConnectionInput) (*usecase.MasterCardConnectionOutput, error) {
	s.gotListInput = in
	return s.listOut, s.listErr
}

// TestAdminMaster_Success verifies the resolver maps the MasterWithCount carrier
// to *model.MasterCardgroup including the cardCount field.
func TestAdminMaster_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{adminOut: &usecase.MasterWithCount{
		Master: &domain.MasterCardgroup{
			ID:      "m1",
			Name:    domain.CardgroupName("Deck"),
			Status:  domain.MasterStatusDraft,
			Version: 1,
		},
		CardCount: 5,
	}}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	got, err := qr.AdminMaster(context.Background(), "m1")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "m1", got.ID)
	assert.Equal(t, model.MasterCardgroupStatusDraft, got.Status)
	assert.Equal(t, 5, got.CardCount)
}

// TestAdminMaster_WrapsForbidden verifies a usecase ForbiddenError is wrapped via
// gqlerr.FromUsecaseError into a FORBIDDEN-coded wire error.
func TestAdminMaster_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{adminErr: ucerr.NewForbiddenError("admin only")}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	_, err := qr.AdminMaster(context.Background(), "m1")
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}

// TestAdminMaster_WrapsValidation verifies a usecase ValidationError (e.g. a
// missing master row) is wrapped into a BAD_USER_INPUT wire error.
func TestAdminMaster_WrapsValidation(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{adminErr: ucerr.NewValidationError("id", "master cardgroup not found")}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	_, err := qr.AdminMaster(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeBadUserInput), "want BAD_USER_INPUT wire code")
}

// TestAdminMasterCardsConnection_Success verifies the resolver maps the model
// enums to usecase enums on the way in and the usecase output to the wire
// connection on the way out (edges, totalCount, node fields, cursor encoding).
func TestAdminMasterCardsConnection_Success(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{listOut: &usecase.MasterCardConnectionOutput{
		Cards: []*domain.MasterCard{
			{ID: "c1", MasterCardgroupID: "m1", Front: domain.CardText("front"), Back: domain.CardText("back"), Position: 1},
			{ID: "c2", MasterCardgroupID: "m1", Front: domain.CardText("f2"), Back: domain.CardText("b2"), Position: 2},
		},
		TotalCount: 2,
		HasNext:    true,
		HasPrev:    false,
		StartCur:   "c1",
		EndCur:     "c2",
	}}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	orderBy := model.MasterCardOrderByPosition
	dir := model.SortOrderAsc
	conn, err := qr.AdminMasterCardsConnection(context.Background(), "m1", nil, nil, nil, nil, nil, &orderBy, &dir)
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Input mapping: model enums translated to usecase enums.
	require.NotNil(t, stub.gotListInput.OrderBy)
	assert.Equal(t, usecase.MasterCardOrderByPosition, *stub.gotListInput.OrderBy)
	require.NotNil(t, stub.gotListInput.OrderDirection)
	assert.Equal(t, usecase.SortOrderAsc, *stub.gotListInput.OrderDirection)
	assert.Equal(t, "m1", stub.gotListInput.MasterCardgroupID)

	// Output mapping: edges, node fields, totalCount, pageInfo.
	require.Len(t, conn.Edges, 2)
	assert.Equal(t, 2, conn.TotalCount)
	assert.Equal(t, "c1", conn.Edges[0].Node.ID)
	assert.Equal(t, "front", conn.Edges[0].Node.Front)
	assert.Equal(t, "back", conn.Edges[0].Node.Back)
	assert.Equal(t, 1, conn.Edges[0].Node.Position)
	assert.Equal(t, "m1", conn.Edges[0].Node.MasterCardgroupID)
	require.NotNil(t, conn.PageInfo)
	assert.True(t, conn.PageInfo.HasNextPage)
	assert.False(t, conn.PageInfo.HasPreviousPage)
}

// TestAdminMasterCardsConnection_WrapsForbidden verifies a usecase ForbiddenError
// is wrapped into a FORBIDDEN-coded wire error.
func TestAdminMasterCardsConnection_WrapsForbidden(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{listErr: ucerr.NewForbiddenError("admin only")}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	_, err := qr.AdminMasterCardsConnection(context.Background(), "m1", nil, nil, nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeForbidden), "want FORBIDDEN wire code")
}
