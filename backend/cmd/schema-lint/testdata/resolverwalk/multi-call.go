package resolver

import "context"

type mutationResolver struct {
	CardUC      cardUsecase
	CardgroupUC cardgroupUsecase
}

type cardUsecase interface {
	Create(ctx context.Context, front string) (interface{}, error)
}

type cardgroupUsecase interface {
	Get(ctx context.Context, id string) (interface{}, error)
}

// HandleSwipe demonstrates a resolver that calls two distinct usecase fields.
func (r *mutationResolver) HandleSwipe(ctx context.Context, cardgroupID string, front string) (interface{}, error) {
	cg, err := r.CardgroupUC.Get(ctx, cardgroupID)
	if err != nil {
		return nil, err
	}
	_ = cg
	result, err := r.CardUC.Create(ctx, front)
	if err != nil {
		return nil, err
	}
	return result, nil
}
