package resolver

import (
	"context"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/usecase"
)

func toUserModel(user *domain.User) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{
		ID:          user.ID,
		DisplayName: (*string)(user.DisplayName),
		Bio:         user.Bio.Ptr(),
		AvatarURL:   user.AvatarURL,
		Version:     int(user.Version),
	}
}

func toCardgroupModel(cg *domain.Cardgroup) *model.Cardgroup {
	if cg == nil {
		return nil
	}
	return &model.Cardgroup{
		ID:        cg.ID,
		Name:      cg.Name.String(),
		OwnerID:   cg.OwnerID,
		CreatedAt: cg.CreatedAt,
		UpdatedAt: cg.UpdatedAt,
	}
}

func toCardModel(card *domain.Card) *model.Card {
	if card == nil {
		return nil
	}
	return &model.Card{
		ID:          card.ID,
		Front:       string(card.Front),
		Back:        string(card.Back),
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

// toCEFRLevelModel maps a domain CEFR level to the generated GraphQL enum. The
// bool is false for domain.CEFRUnknown, signalling the resolver to return null.
func toCEFRLevelModel(level domain.CEFRLevel) (model.CEFRLevel, bool) {
	switch level {
	case domain.CEFRA1:
		return model.CEFRLevelA1, true
	case domain.CEFRA2:
		return model.CEFRLevelA2, true
	case domain.CEFRB1:
		return model.CEFRLevelB1, true
	case domain.CEFRB2:
		return model.CEFRLevelB2, true
	case domain.CEFRC1:
		return model.CEFRLevelC1, true
	case domain.CEFRC2:
		return model.CEFRLevelC2, true
	default:
		return "", false
	}
}

func toLearnDisplayModeModel(m domain.LearnDisplayMode) model.LearnDisplayMode {
	if m == domain.LearnDisplayAlwaysVisible {
		return model.LearnDisplayModeAlwaysVisible
	}
	return model.LearnDisplayModeFlipToReveal
}

func fromLearnDisplayModeModel(m model.LearnDisplayMode) (domain.LearnDisplayMode, error) {
	switch m {
	case model.LearnDisplayModeFlipToReveal:
		return domain.LearnDisplayFlipToReveal, nil
	case model.LearnDisplayModeAlwaysVisible:
		return domain.LearnDisplayAlwaysVisible, nil
	default:
		return "", eris.Errorf("resolver: unknown learn display mode %q", m)
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

func toRoleModel(r *domain.Role) *model.Role {
	if r == nil {
		return nil
	}
	return &model.Role{ID: r.ID, Name: string(r.Name)}
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

func toUsecaseCardOrderBy(o *model.CardOrderBy) *usecase.CardOrderBy {
	if o == nil {
		return nil
	}
	v := usecase.CardOrderBy(*o)
	return &v
}

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
