package resolver

import (
	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/usecase"
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
		LastActive:  user.LastActive,
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

func toCardgroupModels(cgs []*domain.Cardgroup) []*model.Cardgroup {
	out := make([]*model.Cardgroup, len(cgs))
	for i, cg := range cgs {
		out[i] = toCardgroupModel(cg)
	}
	return out
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
		Due:         card.FSRS.Due,
		Stability:   card.FSRS.Stability,
		Difficulty:  card.FSRS.Difficulty,
		State:       int(card.FSRS.State),
		Reps:        card.FSRS.Reps,
		Lapses:      card.FSRS.Lapses,
		LastReview:  card.FSRS.LastReview,
		CreatedAt:   card.CreatedAt,
		UpdatedAt:   card.UpdatedAt,
	}
}

func toCardModels(cards []*domain.Card) []*model.Card {
	out := make([]*model.Card, len(cards))
	for i, card := range cards {
		out[i] = toCardModel(card)
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

func toSwipeResponseModel(out *usecase.SwipeOutput) *model.SwipeResponse {
	if out == nil {
		return nil
	}
	return &model.SwipeResponse{
		NextCards:       toCardModels(out.NextCards),
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

// toCardConnectionModel emits cursors as bare card UUIDs (no base64).
func toCardConnectionModel(out *usecase.CardConnectionOutput) *model.CardConnection {
	if out == nil {
		return &model.CardConnection{Edges: []*model.CardEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := make([]*model.CardEdge, len(out.Cards))
	for i, c := range out.Cards {
		edges[i] = &model.CardEdge{Cursor: c.ID, Node: toCardModel(c)}
	}
	return &model.CardConnection{
		Edges: edges,
		PageInfo: &model.PageInfo{
			HasNextPage:     out.HasNext,
			HasPreviousPage: out.HasPrev,
			StartCursor:     nilIfEmpty(out.StartCur),
			EndCursor:       nilIfEmpty(out.EndCur),
		},
		TotalCount: int(out.TotalCount),
	}
}

// toCardgroupConnectionModel emits cursors as bare cardgroup UUIDs (no
// base64). Mirrors toCardConnectionModel for the cardgroup aggregate.
func toCardgroupConnectionModel(out *usecase.CardgroupConnectionOutput) *model.CardgroupConnection {
	if out == nil {
		return &model.CardgroupConnection{Edges: []*model.CardgroupEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := make([]*model.CardgroupEdge, len(out.Cardgroups))
	for i, cg := range out.Cardgroups {
		edges[i] = &model.CardgroupEdge{Cursor: cg.ID, Node: toCardgroupModel(cg)}
	}
	return &model.CardgroupConnection{
		Edges: edges,
		PageInfo: &model.PageInfo{
			HasNextPage:     out.HasNext,
			HasPreviousPage: out.HasPrev,
			StartCursor:     nilIfEmpty(out.StartCur),
			EndCursor:       nilIfEmpty(out.EndCur),
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

func toRoleModels(roles []*domain.Role) []*model.Role {
	out := make([]*model.Role, len(roles))
	for i, r := range roles {
		out[i] = toRoleModel(r)
	}
	return out
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// toFSRSOverride collects the nine optional FSRS pointer fields from
// model.NewCardInput into a domain.FSRSStateOverride. Returns nil when every
// field is nil so the usecase can take the default-FSRS branch.
func toFSRSOverride(in model.NewCardInput) *domain.FSRSStateOverride {
	if in.Due == nil && in.Stability == nil && in.Difficulty == nil &&
		in.ElapsedDays == nil && in.ScheduledDays == nil && in.Reps == nil &&
		in.Lapses == nil && in.State == nil && in.LastReview == nil {
		return nil
	}
	return &domain.FSRSStateOverride{
		Due:           in.Due,
		Stability:     in.Stability,
		Difficulty:    in.Difficulty,
		ElapsedDays:   in.ElapsedDays,
		ScheduledDays: in.ScheduledDays,
		Reps:          in.Reps,
		Lapses:        in.Lapses,
		State:         in.State,
		LastReview:    in.LastReview,
	}
}
