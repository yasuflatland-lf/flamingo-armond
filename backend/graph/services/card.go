// card.go

package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
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

func (c *cardService) GetCardByID(ctx context.Context, id int64) (*model.Card, error) {
	var card repository.Card
	if err := c.db.WithContext(ctx).First(&card, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("card not found")
		}
		return nil, err
	}
	return convertToCard(card), nil
}

func (c *cardService) CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error) {
	gormCard := convertToGormCard(input)
	result := c.db.WithContext(ctx).Create(gormCard)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "foreign key constraint") {
			return nil, fmt.Errorf("invalid card group ID")
		}
		return nil, result.Error
	}
	return convertToCard(*gormCard), nil
}

func (c *cardService) UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error) {
	var card repository.Card
	if err := c.db.WithContext(ctx).First(&card, id).Error; err != nil {
		return nil, err
	}
	card.Front = input.Front
	card.Back = input.Back
	card.ReviewDate = input.ReviewDate
	card.IntervalDays = func() int {
		if input.IntervalDays != nil {
			return *input.IntervalDays
		}
		return card.IntervalDays
	}()
	card.Updated = time.Now()
	if err := c.db.WithContext(ctx).Save(&card).Error; err != nil {
		return nil, err
	}
	return convertToCard(card), nil
}

func (c *cardService) DeleteCard(ctx context.Context, id int64) (bool, error) {
	result := c.db.WithContext(ctx).Delete(&repository.Card{}, id)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, fmt.Errorf("record not found")
	}
	return true, nil
}

func (c *cardService) Cards(ctx context.Context) ([]*model.Card, error) {
	var cards []repository.Card
	if err := c.db.WithContext(ctx).Find(&cards).Error; err != nil {
		return nil, err
	}
	var gqlCards []*model.Card
	for _, card := range cards {
		gqlCards = append(gqlCards, convertToCard(card))
	}
	return gqlCards, nil
}

func (c *cardService) CardsByCardGroup(ctx context.Context, cardGroupID int64) ([]*model.Card, error) {
	var cards []repository.Card
	if err := c.db.WithContext(ctx).Where("cardgroup_id = ?", cardGroupID).Find(&cards).Error; err != nil {
		return nil, err
	}
	var gqlCards []*model.Card
	for _, card := range cards {
		gqlCards = append(gqlCards, convertToCard(card))
	}
	return gqlCards, nil
}
