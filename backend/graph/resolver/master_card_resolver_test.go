package resolver

import (
	"context"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/cursor"
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

// ---------------------------------------------------------------------------
// UNAUTHENTICATED wrap (test item 1)
// ---------------------------------------------------------------------------

// TestAdminMaster_WrapsUnauthenticated verifies a usecase ErrUnauthenticated
// is wrapped via gqlerr.FromUsecaseError into an UNAUTHENTICATED-coded wire error.
func TestAdminMaster_WrapsUnauthenticated(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{adminErr: ucerr.ErrUnauthenticated}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	_, err := qr.AdminMaster(context.Background(), "m1")
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeUnauthenticated), "want UNAUTHENTICATED wire code")
}

// TestAdminMasterCardsConnection_WrapsUnauthenticated verifies a usecase
// ErrUnauthenticated is wrapped into an UNAUTHENTICATED-coded wire error.
func TestAdminMasterCardsConnection_WrapsUnauthenticated(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{listErr: ucerr.ErrUnauthenticated}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	_, err := qr.AdminMasterCardsConnection(context.Background(), "m1", nil, nil, nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeUnauthenticated), "want UNAUTHENTICATED wire code")
}

// ---------------------------------------------------------------------------
// INTERNAL wrap (test item 2)
// ---------------------------------------------------------------------------

// TestAdminMaster_WrapsInternal verifies that an opaque infra error (non-typed,
// eris-wrapped) is promoted to an INTERNAL-coded wire error by FromUsecaseError.
func TestAdminMaster_WrapsInternal(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{adminErr: eris.New("usecase: db: connection reset")}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	_, err := qr.AdminMaster(context.Background(), "m1")
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeInternal), "want INTERNAL wire code")
}

// TestAdminMasterCardsConnection_WrapsInternal verifies that an opaque infra
// error is promoted to an INTERNAL-coded wire error.
func TestAdminMasterCardsConnection_WrapsInternal(t *testing.T) {
	t.Parallel()
	stub := &stubMasterCardUC{listErr: eris.New("usecase: db: query timeout")}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	_, err := qr.AdminMasterCardsConnection(context.Background(), "m1", nil, nil, nil, nil, nil, nil, nil)
	require.Error(t, err)
	assert.True(t, gqlerr.IsCode(err, gqlerr.CodeInternal), "want INTERNAL wire code")
}

// ---------------------------------------------------------------------------
// Non-nil input translation (test item 3)
// ---------------------------------------------------------------------------

// TestAdminMasterCardsConnection_TranslatesNonNilInputs verifies that model
// enum arguments are translated to the correct usecase enum values before being
// passed to the usecase. This is the resolver's sole translation responsibility
// and is not covered by the usecase-layer tests.
func TestAdminMasterCardsConnection_TranslatesNonNilInputs(t *testing.T) {
	t.Parallel()

	first := 5
	afterCur := cursor.Encode("after-id")
	search := "foo"
	orderBy := model.MasterCardOrderByCreatedAt
	dir := model.SortOrderDesc

	stub := &stubMasterCardUC{listOut: &usecase.MasterCardConnectionOutput{}}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	_, err := qr.AdminMasterCardsConnection(
		context.Background(),
		"m1",
		&first,
		&afterCur,
		nil,
		nil,
		&search,
		&orderBy,
		&dir,
	)
	require.NoError(t, err)

	in := stub.gotListInput
	assert.Equal(t, "m1", in.MasterCardgroupID, "MasterCardgroupID must pass through")

	require.NotNil(t, in.OrderBy, "OrderBy must be non-nil when provided")
	assert.Equal(t, usecase.MasterCardOrderByCreatedAt, *in.OrderBy, "OrderBy must translate to usecase enum")

	require.NotNil(t, in.OrderDirection, "OrderDirection must be non-nil when provided")
	assert.Equal(t, usecase.SortOrderDesc, *in.OrderDirection, "OrderDirection must translate to usecase enum")

	require.NotNil(t, in.Search, "Search must be non-nil when provided")
	assert.Equal(t, "foo", *in.Search, "Search must pass through unchanged")

	require.NotNil(t, in.First, "First must be non-nil when provided")
	assert.Equal(t, 5, *in.First, "First must pass through as-is")

	require.NotNil(t, in.After, "After must be non-nil when provided")
	assert.Equal(t, afterCur, *in.After, "After must pass through unchanged")
}

// ---------------------------------------------------------------------------
// PageInfo cursor single-encode regression (test item 4 / C1 fix guard)
// ---------------------------------------------------------------------------

// TestAdminMasterCardsConnection_CursorRoundTrip asserts that the resolver
// applies cursor.Encode exactly once to the raw IDs returned by the usecase
// (the C1 fix: prevent double-encoding). StartCursor, EndCursor, and each
// edge.Cursor must all decode back to the original raw ID.
func TestAdminMasterCardsConnection_CursorRoundTrip(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCardUC{listOut: &usecase.MasterCardConnectionOutput{
		Cards: []*domain.MasterCard{
			{ID: "first-id", MasterCardgroupID: "m1", Front: domain.CardText("F"), Back: domain.CardText("B"), Position: 1},
			{ID: "last-id", MasterCardgroupID: "m1", Front: domain.CardText("F2"), Back: domain.CardText("B2"), Position: 2},
		},
		TotalCount: 2,
		HasNext:    false,
		HasPrev:    false,
		StartCur:   "first-id",
		EndCur:     "last-id",
	}}
	qr := &queryResolver{&Resolver{MasterCardUC: stub}}

	conn, err := qr.AdminMasterCardsConnection(context.Background(), "m1", nil, nil, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.Len(t, conn.Edges, 2)

	// StartCursor must decode to "first-id" (singly encoded).
	require.NotNil(t, conn.PageInfo.StartCursor)
	startDecoded, decErr := cursor.Decode(*conn.PageInfo.StartCursor)
	require.NoError(t, decErr, "StartCursor must be valid v1 cursor")
	assert.Equal(t, "first-id", startDecoded, "StartCursor must decode to raw ID — double-encode would produce a wrong value")

	// EndCursor must decode to "last-id".
	require.NotNil(t, conn.PageInfo.EndCursor)
	endDecoded, decErr := cursor.Decode(*conn.PageInfo.EndCursor)
	require.NoError(t, decErr, "EndCursor must be valid v1 cursor")
	assert.Equal(t, "last-id", endDecoded, "EndCursor must decode to raw ID")

	// Each edge cursor must decode to its node's ID.
	for _, edge := range conn.Edges {
		decoded, decErr := cursor.Decode(edge.Cursor)
		require.NoError(t, decErr, "edge.Cursor for node %s must be valid v1 cursor", edge.Node.ID)
		assert.Equal(t, edge.Node.ID, decoded, "edge.Cursor must decode to node.ID")
	}
}
