package usecase

import (
	"context"

	"backend/internal/usecase/ucerr"
)

type reportUsecase struct{}

// Generate references ucerr.Classify, which is NOT one of the three matched
// emit signals (NewValidationError, NewForbiddenError, ErrUnauthenticated).
// The walker must set EmitsTypedError = false for this method.
func (u *reportUsecase) Generate(ctx context.Context) error {
	return ucerr.Classify(nil)
}
