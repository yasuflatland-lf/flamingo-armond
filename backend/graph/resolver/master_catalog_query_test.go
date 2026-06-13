package resolver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
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
}

func (s *stubMasterCatalogUC) ListPublishedConnection(
	_ context.Context, in usecase.MasterCatalogConnectionInput,
) (*usecase.MasterCatalogConnectionOutput, error) {
	s.gotInput = in
	return s.out, s.err
}

// TestQueryResolver_MasterCatalog_Success verifies the resolver maps the model
// enums to usecase enums on the way in and the usecase output to the wire
// connection on the way out.
func TestQueryResolver_MasterCatalog_Success(t *testing.T) {
	t.Parallel()

	stub := &stubMasterCatalogUC{
		out: &usecase.MasterCatalogConnectionOutput{
			Items: []*repository.MasterCatalogItem{
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
			assert.True(t, gqlerr.IsCode(err, tc.want), "want wire code %s", tc.want)
		})
	}
}
