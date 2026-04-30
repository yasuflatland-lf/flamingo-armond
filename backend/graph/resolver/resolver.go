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

// NewResolver wires every Resolver dependency at construction time. All
// arguments are required; pass nil only in test code that explicitly
// asserts the dependency is unused, and use a named test helper rather
// than calling NewResolver with bare nils.
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
