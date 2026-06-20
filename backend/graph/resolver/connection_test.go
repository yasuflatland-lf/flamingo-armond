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
)

// buildEdges is the generic edge mapper shared by the four to*ConnectionModel
// shims. The shims are covered end-to-end in helpers_test.go and
// master_catalog_mapper_test.go; these cases pin the generic's own contract
// directly: cursor.Encode is applied exactly once per edge, nil nodes are
// skipped, the edge constructor receives the encoded cursor and mapped node,
// and the WarnContext label is the caller-supplied one.

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
// edge per non-nil node with cursor == cursor.Encode(getID(item)) and the
// mapped node threaded through the edge constructor.
func TestBuildEdges_EncodesCursorOncePerEdge(t *testing.T) {
	t.Parallel()

	items := []buildEdgesItem{
		{id: "a", node: &buildEdgesNode{id: "a"}},
		{id: "b", node: &buildEdgesNode{id: "b"}},
	}

	edges := buildEdges(context.Background(), items, "test",
		func(it buildEdgesItem) *buildEdgesNode { return it.node },
		func(it buildEdgesItem) string { return it.id },
		func(cur string, n *buildEdgesNode) *buildEdgesEdge { return &buildEdgesEdge{cursor: cur, node: n} },
	)

	require.Len(t, edges, 2)
	for i, want := range []string{"a", "b"} {
		assert.Equal(t, cursor.Encode(want), edges[i].cursor,
			"edges[%d].cursor must be cursor.Encode(getID(item)) exactly once", i)
		assert.True(t, strings.HasPrefix(edges[i].cursor, "v1:"),
			"edges[%d].cursor should carry the v1 envelope, got %q", i, edges[i].cursor)
		require.NotNil(t, edges[i].node)
		assert.Equal(t, want, edges[i].node.id, "edge node must be the mapped node")
	}
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

	edges := buildEdges(context.Background(), items, "toExampleConnectionModel",
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

	edges := buildEdges(context.Background(), []buildEdgesItem{}, "test",
		func(it buildEdgesItem) *buildEdgesNode { return it.node },
		func(it buildEdgesItem) string { return it.id },
		func(cur string, n *buildEdgesNode) *buildEdgesEdge { return &buildEdgesEdge{cursor: cur, node: n} },
	)

	require.NotNil(t, edges, "buildEdges must return a non-nil empty slice")
	assert.Empty(t, edges)
}
