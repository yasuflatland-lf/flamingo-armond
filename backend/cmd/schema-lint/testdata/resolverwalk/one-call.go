package resolver

import "context"

type mutationResolver struct {
	AdminRoleUC adminRoleUsecase
}

type adminRoleUsecase interface {
	Update(ctx context.Context, id, name string) (interface{}, error)
}

func (r *mutationResolver) UpdateRole(ctx context.Context, id string, name string) (interface{}, error) {
	result, err := r.AdminRoleUC.Update(ctx, id, name)
	if err != nil {
		return nil, err
	}
	return result, nil
}
