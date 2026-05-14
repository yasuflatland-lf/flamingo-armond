package resolver

import (
	"context"
	"log/slog"

	"backend/graph/model"
	"backend/internal/cursor"
	"backend/internal/domain"
	"backend/internal/usecase"

	"github.com/rotisserie/eris"
)

// Helpers live in a separate file so `gqlgen generate` does not strip them
// when regenerating schema.resolvers.go (gqlgen only preserves resolver
// methods, not top-level helper functions, in the managed file).
func toUserModel(user *domain.User) *model.User {
	if user == nil {
		return nil
	}
	out := &model.User{
		ID:          user.ID,
		DisplayName: user.DisplayName,
		Bio:         user.Bio,
		AvatarURL:   user.AvatarURL,
	}
	// LastViewedCardgroup carries only the ID across the model boundary; the
	// userResolver.LastViewedCardgroup field resolver hydrates the rest via the
	// per-request Cardgroup DataLoader. Storing the ID this way avoids exposing
	// a flat lastViewedCardgroupId field on the User type while still letting
	// the resolver branch on a non-nil obj.LastViewedCardgroup.
	if user.LastViewedCardgroupID != nil {
		out.LastViewedCardgroup = &model.Cardgroup{ID: *user.LastViewedCardgroupID}
	}
	return out
}

// toCardgroupModel leaves Owner nil; cardgroupResolver.Owner populates it
// lazily via the per-request User DataLoader.
func toCardgroupModel(cg *domain.Cardgroup) *model.Cardgroup {
	if cg == nil {
		return nil
	}
	return &model.Cardgroup{
		ID:        cg.ID,
		Name:      cg.Name,
		OwnerID:   cg.OwnerID,
		CreatedAt: cg.CreatedAt,
		UpdatedAt: cg.UpdatedAt,
	}
}

// toCardModel leaves Cardgroup nil; cardResolver.Cardgroup populates it lazily
// via the per-request Cardgroup DataLoader.
func toCardModel(card *domain.Card) *model.Card {
	if card == nil {
		return nil
	}
	return &model.Card{
		ID:          card.ID,
		Front:       card.Front,
		Back:        card.Back,
		CardgroupID: card.CardgroupID,
		CreatedAt:   card.CreatedAt,
		UpdatedAt:   card.UpdatedAt,
	}
}

func toModelUserCardState(ucs *domain.UserCardFSRS) *model.UserCardState {
	if ucs == nil {
		return nil
	}
	return &model.UserCardState{
		Due:           ucs.State.Due,
		Stability:     ucs.State.Stability,
		Difficulty:    ucs.State.Difficulty,
		State:         int(ucs.State.State),
		Reps:          ucs.State.Reps,
		Lapses:        ucs.State.Lapses,
		LastReview:    ucs.State.LastReview,
		ElapsedDays:   ucs.State.ElapsedDays,
		ScheduledDays: ucs.State.ScheduledDays,
	}
}

func toCardModels(ctx context.Context, cards []*domain.Card) []*model.Card {
	out := make([]*model.Card, 0, len(cards))
	for _, card := range cards {
		cm := toCardModel(card)
		if cm == nil {
			slog.WarnContext(ctx, "toCardModels: skipping nil entry")
			continue
		}
		out = append(out, cm)
	}
	return out
}

// The gqlgen-generated and usecase enums share identical string values
// ("ID", "CREATED_AT", …) so conversion is a direct cast.
func toUsecaseCardOrderBy(o *model.CardOrderBy) *usecase.CardOrderBy {
	if o == nil {
		return nil
	}
	v := usecase.CardOrderBy(*o)
	return &v
}

// toUsecaseCardgroupOrderBy mirrors toUsecaseCardOrderBy for the
// CardgroupOrderBy enum (string values "ID", "CREATED_AT", "UPDATED_AT",
// "NAME").
func toUsecaseCardgroupOrderBy(o *model.CardgroupOrderBy) *usecase.CardgroupOrderBy {
	if o == nil {
		return nil
	}
	v := usecase.CardgroupOrderBy(*o)
	return &v
}

func toUsecaseSortOrder(d *model.SortOrder) *usecase.SortOrder {
	if d == nil {
		return nil
	}
	v := usecase.SortOrder(*d)
	return &v
}

func toSwipeResponseModel(ctx context.Context, out *usecase.SwipeOutput) *model.SwipeResponse {
	if out == nil {
		return nil
	}
	return &model.SwipeResponse{
		NextCards:       toCardModels(ctx, out.NextCards),
		PerformanceMode: out.PerformanceMode,
		Metrics: &model.PerformanceMetrics{
			SuccessRate:   out.Metrics.SuccessRate,
			AvgDifficulty: out.Metrics.AvgDifficulty,
			RetentionRate: out.Metrics.RetentionRate,
			StudyStreak:   out.Metrics.StudyStreak,
			LapseRate:     out.Metrics.LapseRate,
			ReviewCount:   out.Metrics.ReviewCount,
		},
	}
}

// toCardConnectionModel emits cursors as opaque v1 envelopes ("v1:" + base64(uuid)).
// Nil entries in out.Cards are skipped to satisfy the schema's non-null node: Card! constraint.
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
	var startCur, endCur *string
	if out.StartCur != "" {
		s := cursor.Encode(out.StartCur)
		startCur = &s
	}
	if out.EndCur != "" {
		e := cursor.Encode(out.EndCur)
		endCur = &e
	}
	return &model.CardConnection{
		Edges: edges,
		PageInfo: &model.PageInfo{
			HasNextPage:     out.HasNext,
			HasPreviousPage: out.HasPrev,
			StartCursor:     startCur,
			EndCursor:       endCur,
		},
		TotalCount: int(out.TotalCount),
	}
}

// toCardgroupConnectionModel emits cursors as opaque v1 envelopes ("v1:" + base64(uuid)).
// Mirrors toCardConnectionModel for the cardgroup aggregate.
// Nil entries in out.Cardgroups are skipped to satisfy the schema's non-null node: Cardgroup! constraint.
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
		edges = append(edges, &model.CardgroupEdge{Cursor: cursor.Encode(cg.ID), Node: cgm})
	}
	var startCur, endCur *string
	if out.StartCur != "" {
		s := cursor.Encode(out.StartCur)
		startCur = &s
	}
	if out.EndCur != "" {
		e := cursor.Encode(out.EndCur)
		endCur = &e
	}
	return &model.CardgroupConnection{
		Edges: edges,
		PageInfo: &model.PageInfo{
			HasNextPage:     out.HasNext,
			HasPreviousPage: out.HasPrev,
			StartCursor:     startCur,
			EndCursor:       endCur,
		},
		TotalCount: int(out.TotalCount),
	}
}

func toRoleModel(r *domain.Role) *model.Role {
	if r == nil {
		return nil
	}
	return &model.Role{ID: r.ID, Name: r.Name}
}

func toRoleModels(ctx context.Context, roles []*domain.Role) []*model.Role {
	out := make([]*model.Role, 0, len(roles))
	for _, r := range roles {
		rm := toRoleModel(r)
		if rm == nil {
			slog.WarnContext(ctx, "toRoleModels: skipping nil entry")
			continue
		}
		out = append(out, rm)
	}
	return out
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// dictionaryKindOrPanic enforces the "UNKNOWN never escapes the server" contract
// documented on the GraphQL DictionaryValidationKind enum. The caller-side cast is
// total at the type level, but a missed Kind assignment in a future construction
// site would silently emit "UNKNOWN" / "" to the wire. Crash loud instead:
// the gqlgen recover middleware will return a 500 to the client and the structured
// log captures the construction context.
func dictionaryKindOrPanic(ctx context.Context, raw string) model.DictionaryValidationKind {
	if raw == "" || raw == string(model.DictionaryValidationKindUnknown) {
		slog.ErrorContext(ctx, "dictionary: UNKNOWN/empty Kind escaped to resolver — programmer bug",
			"raw", raw,
		)
		panic(eris.Errorf("dictionary: UNKNOWN/empty Kind escaped to resolver: %q", raw))
	}
	return model.DictionaryValidationKind(raw)
}
