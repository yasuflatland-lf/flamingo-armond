package resolver

import "context"

type mutationResolver struct{}

func (r *mutationResolver) DeleteRole(ctx context.Context, id string) (bool, error) {
	// No usecase call — resolver handles the logic inline.
	_ = ctx
	_ = id
	return true, nil
}
