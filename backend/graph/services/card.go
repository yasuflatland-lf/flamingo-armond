package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
)

type cardService struct {
	db *gorm.DB
}

func convertToGormCard(input model.NewCard) *repository.Card {
	return &repository.Card{
		Front:      input.Front,
		Back:       input.Back,
		ReviewDate: input.ReviewDate,
		IntervalDays: func() int {
			if input.IntervalDays != nil {
				return *input.IntervalDays
			}
			return 1
		}(),
		CardGroupID: input.CardgroupID,
		Created:     input.Created,
		Updated:     input.Updated,
	}
}

func convertToCard(card repository.Card) *model.Card {
	return &model.Card{
		ID:           card.ID,
		Front:        card.Front,
		Back:         card.Back,
		ReviewDate:   card.ReviewDate,
		IntervalDays: card.IntervalDays,
		Created:      card.Created,
		Updated:      card.Updated,
	}
}

func (c *cardService) GetUserByID(ctx context.Context, id int64) (*model.Card, error) {
	var card repository.Card
	if err := c.db.WithContext(ctx).First(&card, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("card not found")
		}
		return nil, err
	}
	return convertToCard(card), nil
}
