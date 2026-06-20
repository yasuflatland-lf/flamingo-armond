package resolver

import (
	"context"
	"log/slog"

	"backend/graph/model"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/repository"
	"backend/internal/usecase"
)

func encodeCursor(id string) *string {
	if id == "" {
		return nil
	}
	s := cursor.Encode(id)
	return &s
}

// buildPageInfo assembles the Relay PageInfo from the boundary flags and the
// start/end cursor strings. Every caller passes the raw boundary id through
// encodeCursor, which applies cursor.Encode exactly once: the usecase
// connection output carries RAW ids, never pre-encoded cursors. Encoding in
// the usecase layer would double-encode the PageInfo cursors (see the
// "Cursor encoding happens exactly once" rule in .claude/rules/pagination.md).
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
// an edge whose cursor is cursor.Encode(getID(item)). cursor.Encode is called
// exactly once per edge inside the loop, preserving the "cursor encoding happens
// exactly once" invariant in .claude/rules/pagination.md. The four to*ConnectionModel
// shims below pass their node-mapper, id accessor, and edge constructor; only
// toUserConnectionModel stays separate because its input carries pre-encoded
// cursors rather than raw ids.
func buildEdges[Item any, Node any, Edge any](
	ctx context.Context,
	items []Item,
	label string,
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
		edges = append(edges, mkEdge(cursor.Encode(getID(item)), n))
	}
	return edges
}

func toCardConnectionModel(ctx context.Context, out *usecase.CardConnectionOutput) *model.CardConnection {
	if out == nil {
		return &model.CardConnection{Edges: []*model.CardEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := buildEdges(ctx, out.Cards, "toCardConnectionModel",
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
	edges := buildEdges(ctx, out.Cardgroups, "toCardgroupConnectionModel",
		toCardgroupModel,
		func(cg *domain.Cardgroup) string { return string(cg.ID) },
		func(cur string, n *model.Cardgroup) *model.CardgroupEdge {
			return &model.CardgroupEdge{Cursor: cur, Node: n}
		},
	)
	return &model.CardgroupConnection{
		Edges:      edges,
		PageInfo:   buildPageInfo(out.HasNext, out.HasPrev, encodeCursor(out.StartCur), encodeCursor(out.EndCur)),
		TotalCount: int(out.TotalCount),
	}
}

func toMasterCatalogConnectionModel(ctx context.Context, out *usecase.MasterCatalogConnectionOutput) *model.MasterCatalogConnection {
	if out == nil {
		return &model.MasterCatalogConnection{Edges: []*model.MasterCatalogEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := buildEdges(ctx, out.Items, "toMasterCatalogConnectionModel",
		toMasterCardgroupModel,
		func(item *repository.MasterCatalogItem) string { return item.Cardgroup.ID },
		func(cur string, n *model.MasterCardgroup) *model.MasterCatalogEdge {
			return &model.MasterCatalogEdge{Cursor: cur, Node: n}
		},
	)
	return &model.MasterCatalogConnection{
		Edges:      edges,
		PageInfo:   buildPageInfo(out.HasNext, out.HasPrev, encodeCursor(out.StartCur), encodeCursor(out.EndCur)),
		TotalCount: int(out.TotalCount),
	}
}

func toMasterCardConnectionModel(ctx context.Context, out *usecase.MasterCardConnectionOutput) *model.MasterCardConnection {
	if out == nil {
		return &model.MasterCardConnection{Edges: []*model.MasterCardEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := buildEdges(ctx, out.Cards, "toMasterCardConnectionModel",
		toMasterCardModel,
		func(c *domain.MasterCard) string { return c.ID },
		func(cur string, n *model.MasterCard) *model.MasterCardEdge {
			return &model.MasterCardEdge{Cursor: cur, Node: n}
		},
	)
	return &model.MasterCardConnection{
		Edges:      edges,
		PageInfo:   buildPageInfo(out.HasNext, out.HasPrev, encodeCursor(out.StartCur), encodeCursor(out.EndCur)),
		TotalCount: int(out.TotalCount),
	}
}

func toUserConnectionModel(ctx context.Context, uc *usecase.AdminUserConnection) *model.UserConnection {
	if uc == nil {
		return &model.UserConnection{Edges: []*model.UserEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := make([]*model.UserEdge, 0, len(uc.Edges))
	for _, e := range uc.Edges {
		um := toUserModel(e.Node)
		if um == nil {
			slog.WarnContext(ctx, "toUserConnectionModel: skipping nil entry")
			continue
		}
		edges = append(edges, &model.UserEdge{Cursor: e.Cursor, Node: um})
	}
	return &model.UserConnection{
		Edges:      edges,
		PageInfo:   buildPageInfo(uc.PageInfo.HasNextPage, uc.PageInfo.HasPreviousPage, uc.PageInfo.StartCursor, uc.PageInfo.EndCursor),
		TotalCount: int(uc.TotalCount),
	}
}
