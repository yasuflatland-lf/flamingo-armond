package resolver

import (
	"backend/internal/auth"
	"backend/internal/usecase"
)

type Resolver struct {
	UserUC       *usecase.UserUsecase
	CardgroupUC  *usecase.CardgroupUsecase
	CardUC       *usecase.CardUsecase
	SwipeUC      *usecase.SwipeUsecase
	AuthSvc      *auth.Service
	DictionaryUC usecase.DictionaryUsecase
	AdminUserUC  usecase.AdminUserUsecase
	AdminRoleUC  usecase.AdminRoleUsecase
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
	dictionaryUC usecase.DictionaryUsecase,
	adminUserUC usecase.AdminUserUsecase,
	adminRoleUC usecase.AdminRoleUsecase,
) *Resolver {
	return &Resolver{
		UserUC:       user,
		CardgroupUC:  cardgroupUC,
		CardUC:       cardUC,
		SwipeUC:      swipeUC,
		AuthSvc:      authSvc,
		DictionaryUC: dictionaryUC,
		AdminUserUC:  adminUserUC,
		AdminRoleUC:  adminRoleUC,
	}
}
