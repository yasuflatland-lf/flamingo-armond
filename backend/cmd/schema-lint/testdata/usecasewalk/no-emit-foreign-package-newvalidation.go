package usecase

import (
	"context"

	foreign "backend/internal/repository"
)

type auditUsecase struct{}

// Save calls foreign.NewValidationError where the package alias is NOT ucerr.
// The walker must NOT treat this as an emit signal and must set
// EmitsTypedError = false.
func (u *auditUsecase) Save(ctx context.Context) error {
	return foreign.NewValidationError(nil)
}
