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
