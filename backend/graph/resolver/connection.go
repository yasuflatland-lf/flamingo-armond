package resolver

import (
	"context"
	"log/slog"

	"backend/graph/model"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/usecase"
)

// encodeCursor wraps a raw boundary id in the v1 opaque envelope, or returns
// nil for the empty id that means "this page has no boundary row". Used by the
// connections whose ordering key is immutable (admin users order by created_at)
// or whose mutable-key defect is tracked separately (card).
func encodeCursor(id string) *string {
	return encodeBoundaryCursor(cursor.Encode, id)
}

// encodeBoundaryCursor applies enc to a raw boundary id, or returns nil for the
// empty id that means "this page has no boundary row". Splitting it out lets
// PageInfo share the exact encoder the edges use, so startCursor/endCursor and
// the first/last edge cursor are always byte-identical.
func encodeBoundaryCursor(enc func(string) string, id string) *string {
	if id == "" {
		return nil
	}
	s := enc(id)
	return &s
}

// orderedCursorEncoder returns the per-id encoder for a connection whose
// ordering key is mutable. It emits v2 cursors carrying the ordering the page
// was served under plus the ordering-key value the row held at serve time, so
// the next page fetch compares against that captured value instead of
// re-reading a row the client may have edited in the meantime. keys is the
// usecase output's node-id → serialized-ordering-key map; a node missing from
// it yields an empty key, which is also the correct value when the ordering
// key IS the id.
func orderedCursorEncoder(ord usecase.PageOrdering, keys map[string]string) func(string) string {
	return func(id string) string {
		return cursor.EncodeV2(cursor.Payload{
			ID:        id,
			OrderBy:   ord.OrderBy,
			Direction: ord.Direction,
			OrderKey:  keys[id],
		})
	}
}

// buildPageInfo assembles the Relay PageInfo from the boundary flags and the
// start/end cursor strings. Every caller passes the raw boundary id through
// encodeCursor / encodeBoundaryCursor, which applies the connection's encoder
// exactly once: the usecase connection output carries RAW ids, never
// pre-encoded cursors. Encoding in the usecase layer would double-encode the
// PageInfo cursors (see the "Cursor encoding happens exactly once" rule in
// .claude/rules/pagination.md).
func buildPageInfo(hasNext, hasPrev bool, start, end *string) *model.PageInfo {
	return &model.PageInfo{
		HasNextPage:     hasNext,
		HasPreviousPage: hasPrev,
		StartCursor:     start,
		EndCursor:       end,
	}
}

// buildEdges maps a usecase output slice into Relay edges. For every item it
// maps the node, skips the item (logging a per-aggregate label) when the node
// is nil so the non-null edge.node schema contract holds, and otherwise appends
// an edge whose cursor is enc(getID(item)). enc is called exactly once per edge
// inside the loop, preserving the "cursor encoding happens exactly once"
// invariant in .claude/rules/pagination.md. All five to*ConnectionModel shims
// below pass their encoder, node-mapper, id accessor, and edge constructor:
// cursor.Encode for the immutable-key connections, orderedCursorEncoder for the
// ones whose ordering key can be edited between two page fetches.
func buildEdges[Item any, Node any, Edge any](
	ctx context.Context,
	items []Item,
	label string,
	enc func(string) string,
	toNode func(Item) *Node,
	getID func(Item) string,
	mkEdge func(cur string, n *Node) *Edge,
) []*Edge {
	edges := make([]*Edge, 0, len(items))
	for _, item := range items {
		n := toNode(item)
		if n == nil {
			slog.WarnContext(ctx, label+": skipping nil entry")
			continue
		}
		edges = append(edges, mkEdge(enc(getID(item)), n))
	}
	return edges
}

func toCardConnectionModel(ctx context.Context, out *usecase.CardConnectionOutput) *model.CardConnection {
	if out == nil {
		return &model.CardConnection{Edges: []*model.CardEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := buildEdges(ctx, out.Cards, "toCardConnectionModel", cursor.Encode,
		toCardModel,
		func(c *domain.Card) string { return c.ID },
		func(cur string, n *model.Card) *model.CardEdge { return &model.CardEdge{Cursor: cur, Node: n} },
	)
	return &model.CardConnection{
		Edges:      edges,
		PageInfo:   buildPageInfo(out.HasNext, out.HasPrev, encodeCursor(out.StartCur), encodeCursor(out.EndCur)),
		TotalCount: int(out.TotalCount),
	}
}

func toCardgroupConnectionModel(ctx context.Context, out *usecase.CardgroupConnectionOutput) *model.CardgroupConnection {
	if out == nil {
		return &model.CardgroupConnection{Edges: []*model.CardgroupEdge{}, PageInfo: &model.PageInfo{}}
	}
	// The cardgroup listing defaults to the mutable UPDATED_AT column, so its
	// cursors must carry the ordering-key value captured at serve time.
	enc := orderedCursorEncoder(out.Ordering, out.OrderKeys)
	edges := buildEdges(ctx, out.Cardgroups, "toCardgroupConnectionModel", enc,
		toCardgroupModel,
		func(cg *domain.Cardgroup) string { return string(cg.ID) },
		func(cur string, n *model.Cardgroup) *model.CardgroupEdge {
			return &model.CardgroupEdge{Cursor: cur, Node: n}
		},
	)
	return &model.CardgroupConnection{
		Edges: edges,
		PageInfo: buildPageInfo(out.HasNext, out.HasPrev,
			encodeBoundaryCursor(enc, out.StartCur), encodeBoundaryCursor(enc, out.EndCur)),
		TotalCount: int(out.TotalCount),
	}
}

func toMasterCatalogConnectionModel(ctx context.Context, out *usecase.MasterCatalogConnectionOutput) *model.MasterCatalogConnection {
	if out == nil {
		return &model.MasterCatalogConnection{Edges: []*model.MasterCatalogEdge{}, PageInfo: &model.PageInfo{}}
	}
	// The catalog defaults to the admin-mutable SORT_ORDER column, so its
	// cursors must carry the ordering-key value captured at serve time.
	enc := orderedCursorEncoder(out.Ordering, out.OrderKeys)
	edges := buildEdges(ctx, out.Items, "toMasterCatalogConnectionModel", enc,
		toMasterCardgroupModel,
		func(item *usecase.MasterCatalogItem) string { return item.Cardgroup.ID },
		func(cur string, n *model.MasterCardgroup) *model.MasterCatalogEdge {
			return &model.MasterCatalogEdge{Cursor: cur, Node: n}
		},
	)
	return &model.MasterCatalogConnection{
		Edges: edges,
		PageInfo: buildPageInfo(out.HasNext, out.HasPrev,
			encodeBoundaryCursor(enc, out.StartCur), encodeBoundaryCursor(enc, out.EndCur)),
		TotalCount: int(out.TotalCount),
	}
}

func toMasterCardConnectionModel(ctx context.Context, out *usecase.MasterCardConnectionOutput) *model.MasterCardConnection {
	if out == nil {
		return &model.MasterCardConnection{Edges: []*model.MasterCardEdge{}, PageInfo: &model.PageInfo{}}
	}
	// The master-card listing defaults to the admin-mutable POSITION column, so
	// its cursors must carry the ordering-key value captured at serve time.
	enc := orderedCursorEncoder(out.Ordering, out.OrderKeys)
	edges := buildEdges(ctx, out.Cards, "toMasterCardConnectionModel", enc,
		toMasterCardModel,
		func(c *domain.MasterCard) string { return c.ID },
		func(cur string, n *model.MasterCard) *model.MasterCardEdge {
			return &model.MasterCardEdge{Cursor: cur, Node: n}
		},
	)
	return &model.MasterCardConnection{
		Edges: edges,
		PageInfo: buildPageInfo(out.HasNext, out.HasPrev,
			encodeBoundaryCursor(enc, out.StartCur), encodeBoundaryCursor(enc, out.EndCur)),
		TotalCount: int(out.TotalCount),
	}
}

func toUserConnectionModel(ctx context.Context, uc *usecase.AdminUserConnection) *model.UserConnection {
	if uc == nil {
		return &model.UserConnection{Edges: []*model.UserEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := buildEdges(ctx, uc.Users, "toUserConnectionModel", cursor.Encode,
		toUserModel,
		func(u *domain.User) string { return string(u.ID) },
		func(cur string, n *model.User) *model.UserEdge { return &model.UserEdge{Cursor: cur, Node: n} },
	)
	return &model.UserConnection{
		Edges:      edges,
		PageInfo:   buildPageInfo(uc.HasNext, uc.HasPrev, encodeCursor(uc.StartCur), encodeCursor(uc.EndCur)),
		TotalCount: int(uc.TotalCount),
	}
}
