package resolver

import (
	"backend/graph/model"
	"backend/internal/domain"
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
