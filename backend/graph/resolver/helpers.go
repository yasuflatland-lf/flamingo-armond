package resolver

import (
	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/usecase"
)

// toUserModel lives in a separate file so `gqlgen generate` does not strip it
// when regenerating schema.resolvers.go (gqlgen only preserves resolver methods,
// not top-level helper functions, in the managed file).
func toUserModel(user *domain.User) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{
		ID:          user.ID,
		DisplayName: user.DisplayName,
		Bio:         user.Bio,
		AvatarURL:   user.AvatarURL,
	}
}

// toCardgroupModel converts a domain.Cardgroup to a model.Cardgroup.
// Owner is intentionally left nil; cardgroupResolver.Owner populates it lazily
// via the per-request User DataLoader.
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

// toUsecaseCardOrderBy translates the gqlgen-generated enum into the
// usecase's typed enum. Values are identical strings ("ID", "CREATED_AT", …)
// so the conversion is a direct cast.
func toUsecaseCardOrderBy(o *model.CardOrderBy) *usecase.CardOrderBy {
	if o == nil {
		return nil
	}
	v := usecase.CardOrderBy(*o)
	return &v
}

// toUsecaseSortOrder translates model.SortOrder into the usecase's typed
// SortOrder.
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
	}
}

// toCardConnectionModel converts a usecase.CardConnectionOutput into the
// generated model.CardConnection. Cursors are bare card UUIDs (no base64).
func toCardConnectionModel(out *usecase.CardConnectionOutput) *model.CardConnection {
	if out == nil {
		return &model.CardConnection{Edges: []*model.CardEdge{}, PageInfo: &model.PageInfo{}}
	}
	edges := make([]*model.CardEdge, len(out.Cards))
	for i, c := range out.Cards {
		edges[i] = &model.CardEdge{Cursor: c.ID, Node: toCardModel(c)}
	}
	var startCur, endCur *string
	if out.StartCur != "" {
		s := out.StartCur
		startCur = &s
	}
	if out.EndCur != "" {
		e := out.EndCur
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
