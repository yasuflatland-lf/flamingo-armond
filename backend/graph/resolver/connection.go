package resolver

import (
	"context"
	"log/slog"

	"backend/graph/model"
	"backend/internal/cursor"
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
// already-encoded start/end cursor strings. Each caller computes its own
// start/end (some encode via cursor.Encode, others carry a pre-encoded value)
// and passes the final strings in.
func buildPageInfo(hasNext, hasPrev bool, start, end *string) *model.PageInfo {
	return &model.PageInfo{
		HasNextPage:     hasNext,
		HasPreviousPage: hasPrev,
		StartCursor:     start,
		EndCursor:       end,
	}
}

func toCardConnectionModel(ctx context.Context, out *usecase.CardConnectionOutput) *model.CardConnection {
	if out == nil {
		return &model.CardConnection{Edges: []*model.CardEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := make([]*model.CardEdge, 0, len(out.Cards))
	for _, c := range out.Cards {
		cm := toCardModel(c)
		if cm == nil {
			slog.WarnContext(ctx, "toCardConnectionModel: skipping nil entry")
			continue
		}
		edges = append(edges, &model.CardEdge{Cursor: cursor.Encode(c.ID), Node: cm})
	}
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
	edges := make([]*model.CardgroupEdge, 0, len(out.Cardgroups))
	for _, cg := range out.Cardgroups {
		cgm := toCardgroupModel(cg)
		if cgm == nil {
			slog.WarnContext(ctx, "toCardgroupConnectionModel: skipping nil entry")
			continue
		}
		edges = append(edges, &model.CardgroupEdge{Cursor: cursor.Encode(string(cg.ID)), Node: cgm})
	}
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
	edges := make([]*model.MasterCatalogEdge, 0, len(out.Items))
	for _, item := range out.Items {
		mm := toMasterCardgroupModel(item)
		if mm == nil {
			slog.WarnContext(ctx, "toMasterCatalogConnectionModel: skipping nil entry")
			continue
		}
		edges = append(edges, &model.MasterCatalogEdge{Cursor: cursor.Encode(item.Cardgroup.ID), Node: mm})
	}
	return &model.MasterCatalogConnection{
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
