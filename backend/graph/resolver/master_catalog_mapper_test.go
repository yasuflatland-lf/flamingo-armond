package resolver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

func strPtr(s string) *string { return &s }

// TestToMasterCardgroupStatusModel maps the lowercase domain status to the
// uppercase wire enum, with an unrecognised status surfacing as the empty enum
// so the non-null field still serializes.
func TestToMasterCardgroupStatusModel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   domain.MasterCardgroupStatus
		want model.MasterCardgroupStatus
	}{
		{"published", domain.MasterStatusPublished, model.MasterCardgroupStatusPublished},
		{"draft", domain.MasterStatusDraft, model.MasterCardgroupStatusDraft},
		{"unknown falls back to empty enum", domain.MasterCardgroupStatus("archived"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, toMasterCardgroupStatusModel(tc.in))
		})
	}
}

// TestToMasterCardgroupModel_FullMapping verifies every field crosses to the
// wire model, including the int64 cardCount narrowing and the status enum.
func TestToMasterCardgroupModel_FullMapping(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 6, 13, 1, 2, 3, 0, time.UTC)
	updated := created.Add(time.Hour)
	item := &repository.MasterCatalogItem{
		Cardgroup: &domain.MasterCardgroup{
			ID:               "mcg-1",
			Name:             domain.CardgroupName("Starter Deck"),
			Description:      strPtr("desc"),
			Version:          3,
			Status:           domain.MasterStatusPublished,
			IsDefaultStarter: true,
			SortOrder:        7,
			CreatedAt:        created,
			UpdatedAt:        updated,
		},
		CardCount: 42,
	}

	got := toMasterCardgroupModel(item)
	require.NotNil(t, got)
	assert.Equal(t, "mcg-1", got.ID)
	assert.Equal(t, "Starter Deck", got.Name)
	assert.Equal(t, strPtr("desc"), got.Description)
	assert.Equal(t, 3, got.Version)
	assert.Equal(t, model.MasterCardgroupStatusPublished, got.Status)
	assert.True(t, got.IsDefaultStarter)
	assert.Equal(t, 7, got.SortOrder)
	assert.Equal(t, 42, got.CardCount)
	assert.Equal(t, created, got.CreatedAt)
	assert.Equal(t, updated, got.UpdatedAt)
}

// TestToMasterCardgroupModel_Nil guards both nil shapes the mapper must reject
// so the [MasterCardgroup!] edge list never carries a nil node.
func TestToMasterCardgroupModel_Nil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, toMasterCardgroupModel(nil))
	assert.Nil(t, toMasterCardgroupModel(&repository.MasterCatalogItem{Cardgroup: nil, CardCount: 1}))
}

// TestToMasterCatalogConnectionModel_Nil returns an empty (non-nil) connection
// so the resolver never returns a null connection for a zero-result page.
func TestToMasterCatalogConnectionModel_Nil(t *testing.T) {
	t.Parallel()

	got := toMasterCatalogConnectionModel(context.Background(), nil)
	require.NotNil(t, got)
	assert.Empty(t, got.Edges)
	require.NotNil(t, got.PageInfo)
	assert.Nil(t, got.PageInfo.StartCursor)
	assert.Nil(t, got.PageInfo.EndCursor)
}

// TestToMasterCatalogConnectionModel_Mapping verifies edge construction, opaque
// cursor encoding, pageInfo, and totalCount narrowing.
func TestToMasterCatalogConnectionModel_Mapping(t *testing.T) {
	t.Parallel()

	out := &usecase.MasterCatalogConnectionOutput{
		Items: []*repository.MasterCatalogItem{
			{Cardgroup: &domain.MasterCardgroup{ID: "a", Name: domain.CardgroupName("A"), Status: domain.MasterStatusPublished}, CardCount: 1},
			{Cardgroup: &domain.MasterCardgroup{ID: "b", Name: domain.CardgroupName("B"), Status: domain.MasterStatusPublished}, CardCount: 2},
		},
		TotalCount: 9,
		HasNext:    true,
		HasPrev:    false,
		StartCur:   "a",
		EndCur:     "b",
	}

	got := toMasterCatalogConnectionModel(context.Background(), out)
	require.Len(t, got.Edges, 2)
	// Cursors are opaque; assert they equal the canonical encoding of the node ID.
	assert.Equal(t, cursor.Encode("a"), got.Edges[0].Cursor)
	assert.Equal(t, "a", got.Edges[0].Node.ID)
	assert.Equal(t, cursor.Encode("b"), got.Edges[1].Cursor)
	assert.Equal(t, 9, got.TotalCount)
	assert.True(t, got.PageInfo.HasNextPage)
	assert.False(t, got.PageInfo.HasPreviousPage)
	require.NotNil(t, got.PageInfo.StartCursor)
	assert.Equal(t, cursor.Encode("a"), *got.PageInfo.StartCursor)
	require.NotNil(t, got.PageInfo.EndCursor)
	assert.Equal(t, cursor.Encode("b"), *got.PageInfo.EndCursor)
}

// TestToMasterCatalogConnectionModel_SkipsNilNode exercises the defensive branch
// that drops an item whose Cardgroup is nil, so the [MasterCatalogEdge!] list
// never carries a nil node even if the usecase emits a malformed item.
func TestToMasterCatalogConnectionModel_SkipsNilNode(t *testing.T) {
	t.Parallel()

	out := &usecase.MasterCatalogConnectionOutput{
		Items: []*repository.MasterCatalogItem{
			{Cardgroup: nil, CardCount: 0},
			{Cardgroup: &domain.MasterCardgroup{ID: "b", Name: domain.CardgroupName("B"), Status: domain.MasterStatusPublished}, CardCount: 2},
		},
		TotalCount: 1,
	}

	got := toMasterCatalogConnectionModel(context.Background(), out)
	require.Len(t, got.Edges, 1)
	assert.Equal(t, "b", got.Edges[0].Node.ID)
}

// TestToUsecaseMasterCatalogOrderBy casts the model enum to the usecase enum and
// passes nil through (absent argument keeps the usecase default).
func TestToUsecaseMasterCatalogOrderBy(t *testing.T) {
	t.Parallel()

	assert.Nil(t, toUsecaseOrderBy[model.MasterCatalogOrderBy, usecase.MasterCatalogOrderBy](nil))

	in := model.MasterCatalogOrderByName
	got := toUsecaseOrderBy[model.MasterCatalogOrderBy, usecase.MasterCatalogOrderBy](&in)
	require.NotNil(t, got)
	assert.Equal(t, usecase.MasterCatalogOrderByName, *got)
}
