package resolver

import (
	"backend/internal/auth"
	"backend/internal/usecase"
)

type Resolver struct {
	User        *usecase.UserUsecase
	CardgroupUC *usecase.CardgroupUsecase
	CardUC      *usecase.CardUsecase
	SwipeUC     *usecase.SwipeUsecase
	AuthSvc     *auth.Service
}

// NewResolver wires every Resolver dependency at construction time. Tests
// may pass nil for unused dependencies; do not pass nil from production
// wiring.
func NewResolver(
	user *usecase.UserUsecase,
	cardgroupUC *usecase.CardgroupUsecase,
	cardUC *usecase.CardUsecase,
	swipeUC *usecase.SwipeUsecase,
	authSvc *auth.Service,
) *Resolver {
	return &Resolver{
		User:        user,
		CardgroupUC: cardgroupUC,
		CardUC:      cardUC,
		SwipeUC:     swipeUC,
		AuthSvc:     authSvc,
	}
}
