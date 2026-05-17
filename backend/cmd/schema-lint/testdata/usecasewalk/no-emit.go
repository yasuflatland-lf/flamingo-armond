package usecase

import (
	"context"

	"github.com/rotisserie/eris"
)

type learnUsecase struct{}

func (u *learnUsecase) NextDueCards(ctx context.Context, cardgroupID string) ([]string, error) {
	if cardgroupID == "" {
		return nil, eris.New("usecasewalk: cardgroupID must not be empty")
	}
	return []string{}, nil
}
