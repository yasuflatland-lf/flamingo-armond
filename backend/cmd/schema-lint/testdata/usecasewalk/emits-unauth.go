package usecase

import (
	"context"

	"backend/internal/usecase/ucerr"
)

type userUsecase struct{}

func (u *userUsecase) Me(ctx context.Context) (interface{}, error) {
	// Simulate an unauthenticated path by referencing the sentinel.
	return nil, ucerr.ErrUnauthenticated
}
