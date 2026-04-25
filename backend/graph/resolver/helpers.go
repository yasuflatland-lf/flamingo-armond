package resolver

import (
	"backend/graph/model"
	"backend/internal/domain"
)

// toUserModel lives in a separate file so `gqlgen generate` does not strip it
// when regenerating schema.resolvers.go (gqlgen only preserves resolver methods,
// not top-level helper functions, in the managed file).
func toUserModel(p *domain.Profile) *model.User {
	if p == nil {
		return nil
	}
	return &model.User{
		ID:          p.ID,
		DisplayName: p.DisplayName,
		Bio:         p.Bio,
		AvatarURL:   p.AvatarURL,
	}
}
