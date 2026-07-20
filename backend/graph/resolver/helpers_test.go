package resolver

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/rotisserie/eris"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/graph/model"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/loader"
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
		CardgroupID: domain.CardgroupID("cg1"),
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
// each edge cursor and both PageInfo cursors in the v2 opaque envelope ("v2:"),
// carrying the page's ordering plus the per-row ordering-key value the usecase
// captured at serve time. The card listing's DEFAULT ordering key is the
// immutable id, but its opt-in DUE / UPDATED_AT orderings both move, so an
// id-only v1 cursor would shift whenever the row it points at is edited or
// reviewed.
func TestToCardConnectionModel_Cursors(t *testing.T) {
	t.Parallel()

	c1 := &domain.Card{ID: "c1", Front: "Q1", Back: "A1", CardgroupID: domain.CardgroupID("cg1")}
	c2 := &domain.Card{ID: "c2", Front: "Q2", Back: "A2", CardgroupID: domain.CardgroupID("cg1")}
	out := &usecase.CardConnectionOutput{
		Cards:    []*domain.Card{c1, c2},
		StartCur: "c1",
		EndCur:   "c2",
		HasNext:  true,
		HasPrev:  false,
		Ordering: usecase.PageOrdering{OrderBy: "due", Direction: "ASC"},
		OrderKeys: map[string]string{
			"c1": "2026-07-20T00:00:00Z",
			"c2": "2026-07-20T01:00:00Z",
		},
	}

	conn := toCardConnectionModel(context.Background(), out)

	assert.Len(t, conn.Edges, 2)
	wantKeys := []string{"2026-07-20T00:00:00Z", "2026-07-20T01:00:00Z"}
	for i, edge := range conn.Edges {
		assert.True(t, strings.HasPrefix(edge.Cursor, "v2:"),
			"edges[%d].Cursor should start with \"v2:\", got %q", i, edge.Cursor)
		decoded, err := cursor.Decode(edge.Cursor)
		require.NoError(t, err)
		assert.Equal(t, cursor.Payload{
			ID:          edge.Node.ID,
			HasOrdering: true,
			OrderBy:     "due",
			Direction:   "ASC",
			OrderKey:    wantKeys[i],
		}, decoded, "edges[%d].Cursor must carry the captured ordering key", i)
	}
	require.NotNil(t, conn.PageInfo.StartCursor)
	require.NotNil(t, conn.PageInfo.EndCursor)
	// PageInfo shares the edges' encoder, so the boundary cursors are
	// byte-identical to the first/last edge cursor.
	assert.Equal(t, conn.Edges[0].Cursor, *conn.PageInfo.StartCursor)
	assert.Equal(t, conn.Edges[1].Cursor, *conn.PageInfo.EndCursor)
}

// TestToCardConnectionModel_EmptyCursors verifies that empty StartCur/EndCur
// result in nil PageInfo cursors.
func TestToCardConnectionModel_EmptyCursors(t *testing.T) {
	t.Parallel()

	c1 := &domain.Card{ID: "c1", Front: "Q1", Back: "A1", CardgroupID: domain.CardgroupID("cg1")}
	out := &usecase.CardConnectionOutput{
		Cards:    []*domain.Card{c1},
		StartCur: "",
		EndCur:   "",
	}

	conn := toCardConnectionModel(context.Background(), out)

	assert.Nil(t, conn.PageInfo.StartCursor, "pageInfo.startCursor should be nil for empty StartCur")
	assert.Nil(t, conn.PageInfo.EndCursor, "pageInfo.endCursor should be nil for empty EndCur")
}

// ---------------------------------------------------------------------------
// toCardgroupConnectionModel — cursor encoding assertions (Finding C)
// ---------------------------------------------------------------------------

// TestToCardgroupConnectionModel_Cursors verifies that toCardgroupConnectionModel
// wraps each edge cursor and both PageInfo cursors in the v2 opaque envelope
// ("v2:"), carrying the page's ordering plus the per-row ordering-key value the
// usecase captured at serve time. The cardgroup listing orders by the mutable
// updated_at column, so an id-only v1 cursor would move whenever the row it
// points at is edited.
func TestToCardgroupConnectionModel_Cursors(t *testing.T) {
	t.Parallel()

	cg1 := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), Name: "Alpha", OwnerID: "u1"}
	cg2 := &domain.Cardgroup{ID: domain.CardgroupID("cg2"), Name: "Beta", OwnerID: "u1"}
	out := &usecase.CardgroupConnectionOutput{
		Cardgroups: []*domain.Cardgroup{cg1, cg2},
		StartCur:   "cg1",
		EndCur:     "cg2",
		HasNext:    true,
		HasPrev:    false,
		Ordering:   usecase.PageOrdering{OrderBy: "updated_at", Direction: "DESC"},
		OrderKeys: map[string]string{
			"cg1": "2026-07-20T01:00:00Z",
			"cg2": "2026-07-20T00:00:00Z",
		},
	}

	conn := toCardgroupConnectionModel(context.Background(), out)

	assert.Len(t, conn.Edges, 2)
	wantKeys := []string{"2026-07-20T01:00:00Z", "2026-07-20T00:00:00Z"}
	for i, edge := range conn.Edges {
		assert.True(t, strings.HasPrefix(edge.Cursor, "v2:"),
			"edges[%d].Cursor should start with \"v2:\", got %q", i, edge.Cursor)
		decoded, err := cursor.Decode(edge.Cursor)
		require.NoError(t, err)
		assert.Equal(t, cursor.Payload{
			ID:          edge.Node.ID,
			HasOrdering: true,
			OrderBy:     "updated_at",
			Direction:   "DESC",
			OrderKey:    wantKeys[i],
		}, decoded, "edges[%d].Cursor must carry the captured ordering key", i)
	}
	require.NotNil(t, conn.PageInfo.StartCursor)
	require.NotNil(t, conn.PageInfo.EndCursor)
	// PageInfo shares the edges' encoder, so the boundary cursors are
	// byte-identical to the first/last edge cursor.
	assert.Equal(t, conn.Edges[0].Cursor, *conn.PageInfo.StartCursor)
	assert.Equal(t, conn.Edges[1].Cursor, *conn.PageInfo.EndCursor)
}

// TestToCardgroupConnectionModel_EmptyCursors verifies that empty StartCur/EndCur
// result in nil PageInfo cursors.
func TestToCardgroupConnectionModel_EmptyCursors(t *testing.T) {
	t.Parallel()

	cg1 := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), Name: "Alpha", OwnerID: "u1"}
	out := &usecase.CardgroupConnectionOutput{
		Cardgroups: []*domain.Cardgroup{cg1},
		StartCur:   "",
		EndCur:     "",
	}

	conn := toCardgroupConnectionModel(context.Background(), out)

	assert.Nil(t, conn.PageInfo.StartCursor, "pageInfo.startCursor should be nil for empty StartCur")
	assert.Nil(t, conn.PageInfo.EndCursor, "pageInfo.endCursor should be nil for empty EndCur")
}

// ---------------------------------------------------------------------------
// FiltersNil tests for Connection helpers
// ---------------------------------------------------------------------------

// TestToCardConnectionModel_FiltersNilNodes verifies that toCardConnectionModel
// skips nil domain.Card entries and only emits edges with non-nil nodes.
// This is critical because the schema declares node: Card! (non-null) on
// CardEdge, so a nil node would violate the schema contract.
func TestToCardConnectionModel_FiltersNilNodes(t *testing.T) {
	t.Parallel()

	validCard := &domain.Card{ID: "c1", Front: "Q", Back: "A", CardgroupID: domain.CardgroupID("cg1")}
	out := &usecase.CardConnectionOutput{
		Cards:      []*domain.Card{nil, validCard, nil},
		TotalCount: 1,
	}

	conn := toCardConnectionModel(context.Background(), out)

	require.Len(t, conn.Edges, 1, "expected 1 edge after nil filter")
	assert.NotNil(t, conn.Edges[0].Node, "edge.Node must be non-nil to satisfy schema constraint")
	assert.Equal(t, "c1", conn.Edges[0].Node.ID)
	assert.Equal(t, 1, conn.TotalCount, "TotalCount should pass through from output")
}

// TestToCEFRLevelModel_AllLevels verifies that toCEFRLevelModel maps every known
// domain CEFR level to the correct GraphQL enum constant, and returns ok==false
// for domain.CEFRUnknown.
func TestToCEFRLevelModel_AllLevels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in     domain.CEFRLevel
		want   model.CEFRLevel
		wantOK bool
	}{
		{domain.CEFRA1, model.CEFRLevelA1, true},
		{domain.CEFRA2, model.CEFRLevelA2, true},
		{domain.CEFRB1, model.CEFRLevelB1, true},
		{domain.CEFRB2, model.CEFRLevelB2, true},
		{domain.CEFRC1, model.CEFRLevelC1, true},
		{domain.CEFRC2, model.CEFRLevelC2, true},
		{domain.CEFRUnknown, "", false},
	}
	for _, tc := range cases {
		m, ok := toCEFRLevelModel(tc.in)
		require.Equal(t, tc.wantOK, ok, "level %v", tc.in)
		require.Equal(t, tc.want, m, "level %v", tc.in)
	}
}

// TestToCardgroupConnectionModel_FiltersNilNodes verifies that toCardgroupConnectionModel
// skips nil domain.Cardgroup entries and only emits edges with non-nil nodes.
// This is critical because the schema declares node: Cardgroup! (non-null) on
// CardgroupEdge, so a nil node would violate the schema contract.
func TestToCardgroupConnectionModel_FiltersNilNodes(t *testing.T) {
	t.Parallel()

	validCG := &domain.Cardgroup{ID: domain.CardgroupID("cg1"), Name: "valid", OwnerID: "u1"}
	out := &usecase.CardgroupConnectionOutput{
		Cardgroups: []*domain.Cardgroup{nil, validCG, nil},
		TotalCount: 1,
	}

	conn := toCardgroupConnectionModel(context.Background(), out)

	require.Len(t, conn.Edges, 1, "expected 1 edge after nil filter")
	assert.NotNil(t, conn.Edges[0].Node, "edge.Node must be non-nil to satisfy schema constraint")
	assert.Equal(t, "cg1", conn.Edges[0].Node.ID)
	assert.Equal(t, 1, conn.TotalCount, "TotalCount should pass through from output")
}

// ---------------------------------------------------------------------------
// toMasterCardConnectionModel — nil-node filtering (test item 5)
// ---------------------------------------------------------------------------

// TestToMasterCardConnectionModel_FiltersNilNodes verifies that
// toMasterCardConnectionModel skips nil domain.MasterCard entries and only
// emits edges with non-nil nodes. This is critical because the schema declares
// node: MasterCard! (non-null) on MasterCardEdge, so a nil node would violate
// the schema contract.
func TestToMasterCardConnectionModel_FiltersNilNodes(t *testing.T) {
	t.Parallel()

	validCard := &domain.MasterCard{
		ID:                "mc1",
		MasterCardgroupID: "mg1",
		Front:             domain.CardText("Front"),
		Back:              domain.CardText("Back"),
		Position:          1,
	}
	out := &usecase.MasterCardConnectionOutput{
		Cards:      []*domain.MasterCard{nil, validCard, nil},
		TotalCount: 1,
	}

	conn := toMasterCardConnectionModel(context.Background(), out)

	require.Len(t, conn.Edges, 1, "expected 1 edge after nil filter")
	assert.NotNil(t, conn.Edges[0].Node, "edge.Node must be non-nil to satisfy MasterCardEdge schema constraint")
	assert.Equal(t, "mc1", conn.Edges[0].Node.ID)
	assert.Equal(t, 1, conn.TotalCount, "TotalCount should pass through from output")
}

// ---------------------------------------------------------------------------
// loadersOrInternal — DataLoader registry nil-guard
// ---------------------------------------------------------------------------

// TestLoadersOrInternal_MissingMiddlewareReturnsInternal verifies that a context
// without an installed loader registry yields a non-nil INTERNAL wire error and
// a nil registry.
func TestLoadersOrInternal_MissingMiddlewareReturnsInternal(t *testing.T) {
	t.Parallel()

	loaders, gqlErr := loadersOrInternal(context.Background())

	assert.Nil(t, loaders, "registry must be nil when middleware is not installed")
	require.NotNil(t, gqlErr, "want non-nil error when middleware is not installed")
	assert.True(t, gqlerr.IsCode(gqlErr, gqlerr.CodeInternal),
		"want INTERNAL wire code, got %v", gqlErr)
}

// TestLoadersOrInternal_InstalledReturnsRegistry verifies that an installed
// loader registry is returned with a nil error.
func TestLoadersOrInternal_InstalledReturnsRegistry(t *testing.T) {
	t.Parallel()

	want := &loader.Loaders{}
	ctx := loader.WithContext(context.Background(), want)

	loaders, gqlErr := loadersOrInternal(ctx)

	assert.Nil(t, gqlErr, "want nil error when middleware is installed")
	assert.Same(t, want, loaders, "want the installed registry returned unchanged")
}

// ---------------------------------------------------------------------------
// classifyLoaderErr — CANCELLED vs INTERNAL ladder
// ---------------------------------------------------------------------------

// TestClassifyLoaderErr_CancelledContexts verifies that a cancelled or
// deadline-exceeded context maps to the CANCELLED wire code.
func TestClassifyLoaderErr_CancelledContexts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{"context.Canceled", context.Canceled},
		{"context.DeadlineExceeded", context.DeadlineExceeded},
		{"wrapped context.Canceled", eris.Wrap(context.Canceled, "loader: batch")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := classifyLoaderErr(context.Background(), tc.err, "resolver: test")
			require.NotNil(t, got)
			assert.True(t, gqlerr.IsCode(got, gqlerr.CodeCancelled),
				"want CANCELLED wire code for %s, got %v", tc.name, got)
		})
	}
}

// TestClassifyLoaderErr_GenericErrorIsInternalWithLabel verifies that a generic
// loader error maps to INTERNAL and that the supplied label travels in the
// logged error_chain (the wire message itself is redacted).
func TestClassifyLoaderErr_GenericErrorIsInternalWithLabel(t *testing.T) {
	// Not parallel: mutates the global slog default.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	const label = "resolver: owner"
	got := classifyLoaderErr(context.Background(), errors.New("db down"), label)

	require.NotNil(t, got)
	assert.True(t, gqlerr.IsCode(got, gqlerr.CodeInternal),
		"want INTERNAL wire code for a generic loader error, got %v", got)
	assert.Contains(t, buf.String(), label,
		"expected the wrap label to appear in the logged error_chain, got %q", buf.String())
}

// ---------------------------------------------------------------------------
// newNoVariantSetError — outcome-union "no variant" internal guard
// ---------------------------------------------------------------------------

// TestNewNoVariantSetError_MessageAndCode verifies that newNoVariantSetError emits the exact
// "resolver: <name> has no variant set" message in the logged chain and an
// INTERNAL wire code.
func TestNewNoVariantSetError_MessageAndCode(t *testing.T) {
	// Not parallel: mutates the global slog default.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	got := newNoVariantSetError(context.Background(), "CreateCardOutcome")

	require.NotNil(t, got)
	assert.True(t, gqlerr.IsCode(got, gqlerr.CodeInternal),
		"want INTERNAL wire code, got %v", got)
	assert.Contains(t, buf.String(), "resolver: CreateCardOutcome has no variant set",
		"expected the exact no-variant message in the logged error_chain, got %q", buf.String())
}
