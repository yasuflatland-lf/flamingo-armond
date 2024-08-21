package usecases

import (
	"backend/graph/services"
	"backend/pkg/textdic"
	"backend/pkg/usecases/dictionary_manager"
	"backend/pkg/usecases/swipe_manager"
)

// Usecases interface aggregates all usecases interfaces
type Usecases interface {
	dictionary_manager.DictionaryManagerUsecase
	swipe_manager.SwipeManagerUsecase
}

// usecases struct holds references to all usecases implementations
type usecases struct {
	dictionary_manager.DictionaryManagerUsecase
	swipe_manager.SwipeManagerUsecase
}

// New creates a new instance of Usecases with the provided services
func New(sv services.Services) Usecases {
	return &usecases{
		DictionaryManagerUsecase: dictionary_manager.NewDictionaryManagerUsecase(
			sv.(services.CardService), textdic.NewTextDictionaryService()),
		SwipeManagerUsecase: swipe_manager.NewSwipeManagerUsecase(sv),
	}
}
