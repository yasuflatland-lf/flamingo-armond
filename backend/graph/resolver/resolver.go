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
