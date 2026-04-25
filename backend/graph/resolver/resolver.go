package resolver

import "backend/internal/usecase"

// Resolver is the root dependency-injection container for gqlgen resolvers.
type Resolver struct {
	Profile *usecase.ProfileUsecase
}
