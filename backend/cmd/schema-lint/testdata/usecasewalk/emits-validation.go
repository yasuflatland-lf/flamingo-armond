package usecase

import (
	"context"

	"backend/internal/usecase/ucerr"
)

type cardUsecase struct{}

func (u *cardUsecase) Create(ctx context.Context, front string) (interface{}, error) {
	if front == "" {
		return nil, ucerr.NewValidationError("front", "must not be empty")
	}
	return struct{ Front string }{Front: front}, nil
}
