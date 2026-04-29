package resolver

import "backend/internal/usecase"

type Resolver struct {
	User *usecase.UserUsecase
}
