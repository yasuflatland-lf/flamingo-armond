package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend/internal/auth"
	"backend/internal/domain"
	"backend/internal/gqlerr"
	"backend/internal/repository"
)

type CardRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	FindByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error)
	Create(ctx context.Context, card *domain.Card) error
	Update(ctx context.Context, id string, patch repository.CardUpdate) (*domain.Card, error)
	Delete(ctx context.Context, id string) error
}

type CardgroupRepositoryForCard interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

type CardUsecase struct {
	cardRepo      CardRepository
	cardgroupRepo CardgroupRepositoryForCard
}

func NewCardUsecase(cardRepo CardRepository, cardgroupRepo CardgroupRepositoryForCard) *CardUsecase {
	return &CardUsecase{cardRepo: cardRepo, cardgroupRepo: cardgroupRepo}
}

type CreateCardInput struct {
	CardgroupID string
	Front       string
	Back        string
}

type UpdateCardInput struct {
	Front *string
	Back  *string
}

func (u *CardUsecase) Card(ctx context.Context, id string) (*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	if err := u.authorizeCardgroup(ctx, card.CardgroupID, user.Sub, false); err != nil {
		return nil, err
	}
	return card, nil
}

func (u *CardUsecase) CardsByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	if err := u.authorizeCardgroup(ctx, cardgroupID, user.Sub, true); err != nil {
		return nil, err
	}
	cards, err := u.cardRepo.FindByCardgroup(ctx, cardgroupID)
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return cards, nil
}

func (u *CardUsecase) Create(ctx context.Context, in CreateCardInput) (*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	if err := u.authorizeCardgroup(ctx, in.CardgroupID, user.Sub, true); err != nil {
		return nil, err
	}

	front := strings.TrimSpace(in.Front)
	back := strings.TrimSpace(in.Back)
	now := time.Now().UTC()
	id, err := uuidV7()
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	card := &domain.Card{
		ID:          id,
		CardgroupID: in.CardgroupID,
		Front:       front,
		Back:        back,
		FSRS:        domain.NewFSRSStateForNewCard(now),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := card.Validate(); err != nil {
		return nil, translateCardErr(ctx, err)
	}
	if err := u.cardRepo.Create(ctx, card); err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return card, nil
}

func (u *CardUsecase) Update(ctx context.Context, id string, in UpdateCardInput) (*domain.Card, error) {
	user := auth.UserFrom(ctx)
	if user == nil {
		return nil, gqlerr.Unauthenticated()
	}
	existing, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, gqlerr.Unauthenticated()
		}
		return nil, gqlerr.Internal(ctx, err)
	}
	if err := u.authorizeCardgroup(ctx, existing.CardgroupID, user.Sub, false); err != nil {
		return nil, err
	}

	patch := repository.CardUpdate{}
	candidate := *existing
	if in.Front != nil {
		front := strings.TrimSpace(*in.Front)
		patch.Front = &front
		candidate.Front = front
	}
	if in.Back != nil {
		back := strings.TrimSpace(*in.Back)
		patch.Back = &back
		candidate.Back = back
	}
	if err := candidate.Validate(); err != nil {
		return nil, translateCardErr(ctx, err)
	}

	updated, err := u.cardRepo.Update(ctx, id, patch)
	if err != nil {
		return nil, gqlerr.Internal(ctx, err)
	}
	return updated, nil
}

func (u *CardUsecase) Delete(ctx context.Context, id string) error {
	user := auth.UserFrom(ctx)
	if user == nil {
		return gqlerr.Unauthenticated()
	}
	card, err := u.cardRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return gqlerr.Unauthenticated()
		}
		return gqlerr.Internal(ctx, err)
	}
	if err := u.authorizeCardgroup(ctx, card.CardgroupID, user.Sub, false); err != nil {
		return err
	}
	if err := u.cardRepo.Delete(ctx, id); err != nil {
		return gqlerr.Internal(ctx, err)
	}
	return nil
}

func (u *CardUsecase) authorizeCardgroup(ctx context.Context, id, userID string, missingAsBadInput bool) error {
	cg, err := u.cardgroupRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) && missingAsBadInput {
			return gqlerr.BadUserInput("cardgroupId", "cardgroup not found")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return gqlerr.Unauthenticated()
		}
		return gqlerr.Internal(ctx, err)
	}
	if cg.OwnerID != userID {
		return gqlerr.Unauthenticated()
	}
	return nil
}

func translateCardErr(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrCardCardgroupIDRequired):
		return gqlerr.BadUserInput("cardgroupId", "cardgroupId is required")
	case errors.Is(err, domain.ErrCardFrontRequired):
		return gqlerr.BadUserInput("front", "front is required")
	case errors.Is(err, domain.ErrCardFrontTooLong):
		return gqlerr.BadUserInput("front", fmt.Sprintf("front must be at most %d characters", domain.CardTextMax))
	case errors.Is(err, domain.ErrCardBackRequired):
		return gqlerr.BadUserInput("back", "back is required")
	case errors.Is(err, domain.ErrCardBackTooLong):
		return gqlerr.BadUserInput("back", fmt.Sprintf("back must be at most %d characters", domain.CardTextMax))
	default:
		return gqlerr.Internal(ctx, err)
	}
}
