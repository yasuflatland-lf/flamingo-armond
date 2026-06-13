package resolver

import (
	"backend/internal/auth"
	"backend/internal/usecase"
)

type Resolver struct {
	UserUC                   usecase.UserUsecase
	CardgroupUC              usecase.CardgroupUsecase
	CardUC                   usecase.CardUsecase
	LearnUC                  usecase.LearnUsecase
	SwipeUC                  usecase.SwipeUsecase
	AuthSvc                  *auth.Service
	CardImportUC             usecase.CardImportUsecase
	AdminUserUC              usecase.AdminUserUsecase
	AdminRoleUC              usecase.AdminRoleUsecase
	LastViewedCardgroupUC    usecase.LastViewedCardgroupUsecase
	UpdateLearnDisplayModeUC usecase.UpdateLearnDisplayModeUsecase
	CEFRUC                   usecase.CEFRClassifier
}

// NewResolver wires every Resolver dependency. Tests may pass nil for unused
// dependencies; do not pass nil from production wiring.
func NewResolver(
	user usecase.UserUsecase,
	cardgroupUC usecase.CardgroupUsecase,
	cardUC usecase.CardUsecase,
	swipeUC usecase.SwipeUsecase,
	authSvc *auth.Service,
	cardImportUC usecase.CardImportUsecase,
	adminUserUC usecase.AdminUserUsecase,
	adminRoleUC usecase.AdminRoleUsecase,
	lastViewedCardgroupUC usecase.LastViewedCardgroupUsecase,
	updateLearnDisplayModeUC usecase.UpdateLearnDisplayModeUsecase,
	learnUC usecase.LearnUsecase,
	cefrUC usecase.CEFRClassifier,
) *Resolver {
	return &Resolver{
		UserUC:                   user,
		CardgroupUC:              cardgroupUC,
		CardUC:                   cardUC,
		SwipeUC:                  swipeUC,
		AuthSvc:                  authSvc,
		CardImportUC:             cardImportUC,
		AdminUserUC:              adminUserUC,
		AdminRoleUC:              adminRoleUC,
		LastViewedCardgroupUC:    lastViewedCardgroupUC,
		UpdateLearnDisplayModeUC: updateLearnDisplayModeUC,
		LearnUC:                  learnUC,
		CEFRUC:                   cefrUC,
	}
}
