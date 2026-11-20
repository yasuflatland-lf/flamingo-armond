package resolver

import "backend/internal/usecase"

type Resolver struct {
	UserUC                   usecase.UserUsecase
	CardgroupUC              usecase.CardgroupUsecase
	CardUC                   usecase.CardUsecase
	LearnUC                  usecase.LearnUsecase
	SwipeUC                  usecase.SwipeUsecase
	CardImportUC             usecase.CardImportUsecase
	AdminUserUC              usecase.AdminUserUsecase
	AdminRoleUC              usecase.AdminRoleUsecase
	LastViewedCardgroupUC    usecase.LastViewedCardgroupUsecase
	UpdateLearnDisplayModeUC usecase.UpdateLearnDisplayModeUsecase
	UpdateNewCardRatioUC     usecase.UpdateNewCardRatioUsecase
	CEFRUC                   usecase.CEFRClassifier
	MasterCatalogUC          usecase.MasterCatalogUsecase
	MasterCardUC             usecase.MasterCardUsecase
	StatsUC                  usecase.StatsUsecase
}

// NewResolver wires every Resolver dependency. Tests may pass nil for unused
// dependencies; do not pass nil from production wiring.
func NewResolver(
	user usecase.UserUsecase,
	cardgroupUC usecase.CardgroupUsecase,
	cardUC usecase.CardUsecase,
	swipeUC usecase.SwipeUsecase,
	cardImportUC usecase.CardImportUsecase,
	adminUserUC usecase.AdminUserUsecase,
	adminRoleUC usecase.AdminRoleUsecase,
	lastViewedCardgroupUC usecase.LastViewedCardgroupUsecase,
	updateLearnDisplayModeUC usecase.UpdateLearnDisplayModeUsecase,
	updateNewCardRatioUC usecase.UpdateNewCardRatioUsecase,
	learnUC usecase.LearnUsecase,
	cefrUC usecase.CEFRClassifier,
	masterCatalogUC usecase.MasterCatalogUsecase,
	masterCardUC usecase.MasterCardUsecase,
	statsUC usecase.StatsUsecase,
) *Resolver {
	return &Resolver{
		UserUC:                   user,
		CardgroupUC:              cardgroupUC,
		CardUC:                   cardUC,
		SwipeUC:                  swipeUC,
		CardImportUC:             cardImportUC,
		AdminUserUC:              adminUserUC,
		AdminRoleUC:              adminRoleUC,
		LastViewedCardgroupUC:    lastViewedCardgroupUC,
		UpdateLearnDisplayModeUC: updateLearnDisplayModeUC,
		UpdateNewCardRatioUC:     updateNewCardRatioUC,
		LearnUC:                  learnUC,
		CEFRUC:                   cefrUC,
		MasterCatalogUC:          masterCatalogUC,
		MasterCardUC:             masterCardUC,
		StatsUC:                  statsUC,
	}
}
