package resolver

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"backend/internal/domain"
	"backend/internal/usecase"
)

// TestToRoleModels_FiltersNil verifies that toRoleModels skips nil domain.Role
// values and returns only non-nil results. This is critical because the
// User.roles field declares [Role!]!, so a nil entry in the list would violate
// the schema.
func TestToRoleModels_FiltersNil(t *testing.T) {
	t.Parallel()

	validRole := &domain.Role{ID: "r1", Name: "admin"}
	roles := []*domain.Role{nil, validRole, nil}

	result := toRoleModels(context.Background(), roles)

	assert.Len(t, result, 1)
	if len(result) > 0 {
		assert.Equal(t, "r1", result[0].ID)
		assert.Equal(t, "admin", result[0].Name)
	}
}

// TestToCardModels_FiltersNil verifies that toCardModels skips nil domain.Card
// values and returns only non-nil results. This is critical because
// card list fields declare [Card!]!, so a nil entry would violate
// the schema.
func TestToCardModels_FiltersNil(t *testing.T) {
	t.Parallel()

	validCard := &domain.Card{
		ID:          "c1",
		Front:       "Q",
		Back:        "A",
		CardgroupID: "cg1",
	}
	cards := []*domain.Card{nil, validCard, nil}

	result := toCardModels(context.Background(), cards)

	assert.Len(t, result, 1)
	if len(result) > 0 {
		assert.Equal(t, "c1", result[0].ID)
		assert.Equal(t, "Q", result[0].Front)
		assert.Equal(t, "A", result[0].Back)
	}
}

// TestToRoleModels_AllNil verifies the edge case where all input roles are nil.
func TestToRoleModels_AllNil(t *testing.T) {
	t.Parallel()

	roles := []*domain.Role{nil, nil, nil}
	result := toRoleModels(context.Background(), roles)

	assert.Empty(t, result)
}

// TestToRoleModels_Empty verifies the edge case where the input slice is empty.
func TestToRoleModels_Empty(t *testing.T) {
	t.Parallel()

	roles := []*domain.Role{}
	result := toRoleModels(context.Background(), roles)

	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// toCardConnectionModel — cursor encoding assertions (Finding C)
// ---------------------------------------------------------------------------

// TestToCardConnectionModel_Cursors verifies that toCardConnectionModel wraps
// each edge cursor and PageInfo cursors in the v1 opaque envelope ("v1:").
func TestToCardConnectionModel_Cursors(t *testing.T) {
	t.Parallel()

	c1 := &domain.Card{ID: "c1", Front: "Q1", Back: "A1", CardgroupID: "cg1"}
	c2 := &domain.Card{ID: "c2", Front: "Q2", Back: "A2", CardgroupID: "cg1"}
	out := &usecase.CardConnectionOutput{
		Cards:    []*domain.Card{c1, c2},
		StartCur: "c1",
		EndCur:   "c2",
		HasNext:  true,
		HasPrev:  false,
	}

	conn := toCardConnectionModel(out)

	assert.Len(t, conn.Edges, 2)
	for i, edge := range conn.Edges {
		assert.True(t, strings.HasPrefix(edge.Cursor, "v1:"),
			"edges[%d].Cursor should start with \"v1:\", got %q", i, edge.Cursor)
	}
	assert.NotNil(t, conn.PageInfo.StartCursor)
	assert.NotNil(t, conn.PageInfo.EndCursor)
	assert.True(t, strings.HasPrefix(*conn.PageInfo.StartCursor, "v1:"),
		"pageInfo.startCursor should start with \"v1:\", got %q", *conn.PageInfo.StartCursor)
	assert.True(t, strings.HasPrefix(*conn.PageInfo.EndCursor, "v1:"),
		"pageInfo.endCursor should start with \"v1:\", got %q", *conn.PageInfo.EndCursor)
}

// TestToCardConnectionModel_EmptyCursors verifies that empty StartCur/EndCur
// result in nil PageInfo cursors.
func TestToCardConnectionModel_EmptyCursors(t *testing.T) {
	t.Parallel()

	c1 := &domain.Card{ID: "c1", Front: "Q1", Back: "A1", CardgroupID: "cg1"}
	out := &usecase.CardConnectionOutput{
		Cards:    []*domain.Card{c1},
		StartCur: "",
		EndCur:   "",
	}

	conn := toCardConnectionModel(out)

	assert.Nil(t, conn.PageInfo.StartCursor, "pageInfo.startCursor should be nil for empty StartCur")
	assert.Nil(t, conn.PageInfo.EndCursor, "pageInfo.endCursor should be nil for empty EndCur")
}

// ---------------------------------------------------------------------------
// toCardgroupConnectionModel — cursor encoding assertions (Finding C)
// ---------------------------------------------------------------------------

// TestToCardgroupConnectionModel_Cursors verifies that toCardgroupConnectionModel
// wraps each edge cursor and PageInfo cursors in the v1 opaque envelope ("v1:").
func TestToCardgroupConnectionModel_Cursors(t *testing.T) {
	t.Parallel()

	cg1 := &domain.Cardgroup{ID: "cg1", Name: "Alpha", OwnerID: "u1"}
	cg2 := &domain.Cardgroup{ID: "cg2", Name: "Beta", OwnerID: "u1"}
	out := &usecase.CardgroupConnectionOutput{
		Cardgroups: []*domain.Cardgroup{cg1, cg2},
		StartCur:   "cg1",
		EndCur:     "cg2",
		HasNext:    true,
		HasPrev:    false,
	}

	conn := toCardgroupConnectionModel(out)

	assert.Len(t, conn.Edges, 2)
	for i, edge := range conn.Edges {
		assert.True(t, strings.HasPrefix(edge.Cursor, "v1:"),
			"edges[%d].Cursor should start with \"v1:\", got %q", i, edge.Cursor)
	}
	assert.NotNil(t, conn.PageInfo.StartCursor)
	assert.NotNil(t, conn.PageInfo.EndCursor)
	assert.True(t, strings.HasPrefix(*conn.PageInfo.StartCursor, "v1:"),
		"pageInfo.startCursor should start with \"v1:\", got %q", *conn.PageInfo.StartCursor)
	assert.True(t, strings.HasPrefix(*conn.PageInfo.EndCursor, "v1:"),
		"pageInfo.endCursor should start with \"v1:\", got %q", *conn.PageInfo.EndCursor)
}

// TestToCardgroupConnectionModel_EmptyCursors verifies that empty StartCur/EndCur
// result in nil PageInfo cursors.
func TestToCardgroupConnectionModel_EmptyCursors(t *testing.T) {
	t.Parallel()

	cg1 := &domain.Cardgroup{ID: "cg1", Name: "Alpha", OwnerID: "u1"}
	out := &usecase.CardgroupConnectionOutput{
		Cardgroups: []*domain.Cardgroup{cg1},
		StartCur:   "",
		EndCur:     "",
	}

	conn := toCardgroupConnectionModel(out)

	assert.Nil(t, conn.PageInfo.StartCursor, "pageInfo.startCursor should be nil for empty StartCur")
	assert.Nil(t, conn.PageInfo.EndCursor, "pageInfo.endCursor should be nil for empty EndCur")
}
