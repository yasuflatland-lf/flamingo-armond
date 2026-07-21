package resolver

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"backend/internal/cursor"
	"backend/internal/usecase"
)

// buildEdges is the generic edge mapper shared by the five to*ConnectionModel
// shims. The shims are covered end-to-end in helpers_test.go and
// master_catalog_mapper_test.go; these cases pin the generic's own contract
// directly: the caller-supplied encoder is applied exactly once per edge, nil
// nodes are skipped, the edge constructor receives the encoded cursor and
// mapped node, and the WarnContext label is the caller-supplied one.

type buildEdgesItem struct {
	id   string
	node *buildEdgesNode
}

type buildEdgesNode struct {
	id string
}

type buildEdgesEdge struct {
	cursor string
	node   *buildEdgesNode
}

// TestBuildEdges_EncodesCursorOncePerEdge verifies that buildEdges emits one
// edge per non-nil node with cursor == enc(getID(item)) and the mapped node
// threaded through the edge constructor.
func TestBuildEdges_EncodesCursorOncePerEdge(t *testing.T) {
	t.Parallel()

	items := []buildEdgesItem{
		{id: "a", node: &buildEdgesNode{id: "a"}},
		{id: "b", node: &buildEdgesNode{id: "b"}},
	}

	edges := buildEdges(context.Background(), items, "test", cursor.Encode,
		func(it buildEdgesItem) *buildEdgesNode { return it.node },
		func(it buildEdgesItem) string { return it.id },
		func(cur string, n *buildEdgesNode) *buildEdgesEdge { return &buildEdgesEdge{cursor: cur, node: n} },
	)

	require.Len(t, edges, 2)
	for i, want := range []string{"a", "b"} {
		assert.Equal(t, cursor.Encode(want), edges[i].cursor,
			"edges[%d].cursor must be enc(getID(item)) exactly once", i)
		assert.True(t, strings.HasPrefix(edges[i].cursor, "v1:"),
			"edges[%d].cursor should carry the v1 envelope, got %q", i, edges[i].cursor)
		require.NotNil(t, edges[i].node)
		assert.Equal(t, want, edges[i].node.id, "edge node must be the mapped node")
	}
}

// TestBuildEdges_UsesSuppliedEncoder verifies that buildEdges routes every edge
// cursor through the caller-supplied encoder rather than hardcoding
// cursor.Encode — the seam the mutable-key connections use to emit v2 cursors.
func TestBuildEdges_UsesSuppliedEncoder(t *testing.T) {
	t.Parallel()

	items := []buildEdgesItem{{id: "a", node: &buildEdgesNode{id: "a"}}}
	calls := 0
	enc := func(id string) string {
		calls++
		return "encoded:" + id
	}

	edges := buildEdges(context.Background(), items, "test", enc,
		func(it buildEdgesItem) *buildEdgesNode { return it.node },
		func(it buildEdgesItem) string { return it.id },
		func(cur string, n *buildEdgesNode) *buildEdgesEdge { return &buildEdgesEdge{cursor: cur, node: n} },
	)

	require.Len(t, edges, 1)
	assert.Equal(t, "encoded:a", edges[0].cursor)
	assert.Equal(t, 1, calls, "the encoder must be applied exactly once per edge")
}

// TestOrderedCursorEncoder_EmitsV2WithCapturedKey verifies that the encoder the
// mutable-key connections use embeds the page's ordering plus the row's
// captured ordering-key value, and that a node absent from the key map encodes
// with an empty key (the correct value when the ordering key IS the id).
func TestOrderedCursorEncoder_EmitsV2WithCapturedKey(t *testing.T) {
	t.Parallel()

	enc := orderedCursorEncoder(
		usecase.PageOrdering{OrderBy: "updated_at", Direction: "DESC"},
		map[string]string{"cg-1": "2026-07-20T04:05:06.789012Z"},
	)

	got, err := cursor.Decode(enc("cg-1"))
	require.NoError(t, err)
	assert.Equal(t, cursor.Payload{
		ID:          "cg-1",
		HasOrdering: true,
		OrderBy:     "updated_at",
		Direction:   "DESC",
		OrderKey:    "2026-07-20T04:05:06.789012Z",
	}, got)

	missing, err := cursor.Decode(enc("cg-unknown"))
	require.NoError(t, err)
	assert.Equal(t, "cg-unknown", missing.ID)
	assert.Empty(t, missing.OrderKey)
}

// TestEncodeBoundaryCursor_EmptyIDIsNil verifies that the PageInfo boundary
// encoder maps the empty id (an empty page has no boundary row) to a nil
// cursor rather than encoding the empty string.
func TestEncodeBoundaryCursor_EmptyIDIsNil(t *testing.T) {
	t.Parallel()

	assert.Nil(t, encodeBoundaryCursor(cursor.Encode, ""))
	assert.Nil(t, encodeCursor(""))
	require.NotNil(t, encodeCursor("a"))
	assert.Equal(t, cursor.Encode("a"), *encodeCursor("a"))
}

// TestBuildEdges_SkipsNilNodesWithLabel verifies that buildEdges drops items
// whose toNode returns nil (preserving the non-null edge.node schema contract)
// and logs the caller-supplied label for each skip.
func TestBuildEdges_SkipsNilNodesWithLabel(t *testing.T) {
	// Not parallel: this test swaps the slog default to capture the warning.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	items := []buildEdgesItem{
		{id: "x", node: nil},
		{id: "y", node: &buildEdgesNode{id: "y"}},
		{id: "z", node: nil},
	}

	edges := buildEdges(context.Background(), items, "toExampleConnectionModel", cursor.Encode,
		func(it buildEdgesItem) *buildEdgesNode { return it.node },
		func(it buildEdgesItem) string { return it.id },
		func(cur string, n *buildEdgesNode) *buildEdgesEdge { return &buildEdgesEdge{cursor: cur, node: n} },
	)

	require.Len(t, edges, 1, "nil-node items must be skipped")
	assert.Equal(t, "y", edges[0].node.id)

	logged := buf.String()
	assert.Equal(t, 2, strings.Count(logged, "toExampleConnectionModel: skipping nil entry"),
		"expected one warning per skipped nil node carrying the caller label, got: %s", logged)
}

// TestBuildEdges_EmptyInput verifies that buildEdges returns a non-nil empty
// slice for empty input, never nil.
func TestBuildEdges_EmptyInput(t *testing.T) {
	t.Parallel()

	edges := buildEdges(context.Background(), []buildEdgesItem{}, "test", cursor.Encode,
		func(it buildEdgesItem) *buildEdgesNode { return it.node },
		func(it buildEdgesItem) string { return it.id },
		func(cur string, n *buildEdgesNode) *buildEdgesEdge { return &buildEdgesEdge{cursor: cur, node: n} },
	)

	require.NotNil(t, edges, "buildEdges must return a non-nil empty slice")
	assert.Empty(t, edges)
}
