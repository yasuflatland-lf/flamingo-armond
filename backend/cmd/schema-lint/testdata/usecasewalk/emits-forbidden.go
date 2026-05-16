package usecase

import (
	"context"

	"backend/internal/usecase/ucerr"
)

type adminRoleUsecase struct{}

func (u *adminRoleUsecase) Delete(ctx context.Context, id string) error {
	if id == "system" {
		return ucerr.NewForbiddenError("cannot delete system role")
	}
	return nil
}
