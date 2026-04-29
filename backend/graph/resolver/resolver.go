package resolver

import "backend/internal/usecase"

type Resolver struct {
	User        *usecase.UserUsecase
	CardgroupUC *usecase.CardgroupUsecase
	CardUC      *usecase.CardUsecase
	SwipeUC     *usecase.SwipeUsecase
}
