package resolver

import (
	"context"
	"log/slog"

	"github.com/rotisserie/eris"

	"backend/graph/model"
	"backend/internal/domain"
	"backend/internal/usecase"
)

// toInputValidationError maps a usecase input-validation carrier to the
// generated outcome-union variant. Shared by every promoted mutation resolver
// that surfaces a validation failure as data.
func toInputValidationError(v *usecase.InputValidationInfo) model.InputValidationError {
	return model.InputValidationError{Field: v.Field, Message: v.Message}
}

func toUserModel(user *domain.User) *model.User {
	if user == nil {
		return nil
	}
	return &model.User{
		ID:          string(user.ID),
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
		ID:        string(cg.ID),
		Name:      cg.Name.String(),
		OwnerID:   string(cg.OwnerID),
		CreatedAt: cg.CreatedAt,
		UpdatedAt: cg.UpdatedAt,
	}
}

// toMasterCardgroupModel maps a published-catalog item (a master cardgroup plus
// its card count) to the generated GraphQL model. The status is mapped to the
// uppercase wire enum; an unrecognised status surfaces as the empty enum value
// so the field still serializes.
func toMasterCardgroupModel(item *usecase.MasterCatalogItem) *model.MasterCardgroup {
	if item == nil || item.Cardgroup == nil {
		return nil
	}
	return toMasterCardgroupModelFromParts(item.Cardgroup, int(item.CardCount))
}

// cardCountResolvedElsewhere marks a single-deck mapping whose card count is
// served by masterCardsConnection.totalCount, not this response.
const cardCountResolvedElsewhere = 0

// toMasterCardgroupModelFromParts maps a domain master cardgroup plus a known
// card count to the generated model. Shared by the catalog list path and the
// admin single-entity mutation responses.
func toMasterCardgroupModelFromParts(m *domain.MasterCardgroup, cardCount int) *model.MasterCardgroup {
	if m == nil {
		return nil
	}
	return &model.MasterCardgroup{
		ID:               m.ID,
		Name:             m.Name.String(),
		Description:      m.Description.Ptr(),
		Version:          m.Version,
		Status:           toMasterCardgroupStatusModel(m.Status),
		IsDefaultStarter: m.IsDefaultStarter,
		SortOrder:        m.SortOrder,
		CardCount:        cardCount,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

// toMasterCardgroupStatusModel maps the lowercase domain status to the
// uppercase generated wire enum.
func toMasterCardgroupStatusModel(s domain.MasterCardgroupStatus) model.MasterCardgroupStatus {
	switch s {
	case domain.MasterStatusPublished:
		return model.MasterCardgroupStatusPublished
	case domain.MasterStatusDraft:
		return model.MasterCardgroupStatusDraft
	default:
		return ""
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
		CardgroupID: string(card.CardgroupID),
		CreatedAt:   card.CreatedAt,
		UpdatedAt:   card.UpdatedAt,
	}
}

func toMasterCardModel(card *domain.MasterCard) *model.MasterCard {
	if card == nil {
		return nil
	}
	return &model.MasterCard{
		ID:                card.ID,
		MasterCardgroupID: card.MasterCardgroupID,
		Front:             string(card.Front),
		Back:              string(card.Back),
		Position:          card.Position,
		CreatedAt:         card.CreatedAt,
		UpdatedAt:         card.UpdatedAt,
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
	switch m {
	case domain.LearnDisplayAlwaysVisible:
		return model.LearnDisplayModeAlwaysVisible
	case domain.LearnDisplayFlipToReveal:
		return model.LearnDisplayModeFlipToReveal
	default:
		// Unreachable in production: the DB CHECK constraint and ParseLearnDisplayMode
		// both prevent unknown values from reaching this path. The flip fallback is a
		// deliberate fail-safe to keep reads non-fatal during rolling deploys or
		// unforeseen schema extensions.
		return model.LearnDisplayModeFlipToReveal
	}
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

// toUsecaseOrderBy casts a pointer to a model-layer order-by / sort enum into
// the matching usecase-layer enum, preserving the nil-passes-through contract
// (an absent argument keeps the usecase default). Both enums share a `~string`
// underlying type, so the cast is a direct value conversion. Callers supply the
// type arguments explicitly, e.g.
// toUsecaseOrderBy[model.CardOrderBy, usecase.CardOrderBy](args.OrderBy).
func toUsecaseOrderBy[M ~string, U ~string](o *M) *U {
	if o == nil {
		return nil
	}
	v := U(*o)
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
