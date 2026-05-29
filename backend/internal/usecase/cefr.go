package usecase

import "backend/internal/domain"

// CEFRClassifier is the application-layer port for classifying a card's front
// text into a CEFR level. The resolver depends on this interface (it may not
// import domain/service). *CEFRUsecase implements it, and the domain service
// (*service.CEFRClassifier) satisfies the wrapped dependency.
type CEFRClassifier interface {
	Classify(front string) (domain.CEFRLevel, bool)
}

// CEFRUsecase is the thin application-layer wrapper over the domain classifier.
// It exists so the resolver can reach the classifier without importing
// domain/service (resolver.mayDependOn omits domain_service).
type CEFRUsecase struct {
	classifier CEFRClassifier
}

// NewCEFRUsecase wraps the domain classifier (a *service.CEFRClassifier). It
// panics on a nil classifier — a missing classifier is a wiring bug.
func NewCEFRUsecase(classifier CEFRClassifier) *CEFRUsecase {
	if classifier == nil {
		panic("usecase: CEFRUsecase requires a non-nil classifier")
	}
	return &CEFRUsecase{classifier: classifier}
}

// Classify delegates to the wrapped classifier.
func (u *CEFRUsecase) Classify(front string) (domain.CEFRLevel, bool) {
	return u.classifier.Classify(front)
}
