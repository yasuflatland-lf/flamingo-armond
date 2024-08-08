package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/pkg/logger"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
)

type cardService struct {
	db           *gorm.DB
	defaultLimit int
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

func (s *cardService) GetCardByID(ctx context.Context, id int64) (*model.Card, error) {
	var card repository.Card
	if err := s.db.WithContext(ctx).First(&card, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err := fmt.Errorf("card not found")
			logger.Logger.ErrorContext(ctx, "Card not found:", slog.String("id", fmt.Sprintf("%d", id)))
			return nil, err
		}
		logger.Logger.ErrorContext(ctx, "Failed to get card by ID", err)
		return nil, err
	}
	return convertToCard(card), nil
}

func (s *cardService) CreateCard(ctx context.Context, input model.NewCard) (*model.Card, error) {
	gormCard := convertToGormCard(input)
	result := s.db.WithContext(ctx).Create(gormCard)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "foreign key constraint") {
			err := fmt.Errorf("invalid card group ID")
			logger.Logger.ErrorContext(ctx, "Failed to create card: invalid card group ID", err)
			return nil, err
		}
		logger.Logger.ErrorContext(ctx, "Failed to create card", result.Error)
		return nil, result.Error
	}
	return convertToCard(*gormCard), nil
}

func (s *cardService) UpdateCard(ctx context.Context, id int64, input model.NewCard) (*model.Card, error) {
	var card repository.Card
	if err := s.db.WithContext(ctx).First(&card, id).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Card does not exist", slog.String("id", fmt.Sprintf("%d", id)))
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

	if err := s.db.WithContext(ctx).Save(&card).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to save card", err)
		return nil, err
	}
	return convertToCard(card), nil
}

func (s *cardService) DeleteCard(ctx context.Context, id int64) (*bool, error) {
	result := s.db.WithContext(ctx).Delete(&repository.Card{}, id)
	if result.Error != nil {
		logger.Logger.ErrorContext(ctx, "Failed to delete card", result.Error)
		return nil, result.Error
	}

	success := result.RowsAffected > 0
	if !success {
		err := fmt.Errorf("record not found")
		logger.Logger.ErrorContext(ctx, "Card not found for deletion", err)
		return &success, err
	}

	return &success, nil
}

func (s *cardService) Cards(ctx context.Context) ([]*model.Card, error) {
	var cards []repository.Card
	if err := s.db.WithContext(ctx).Find(&cards).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve cards", err)
		return nil, err
	}
	var gqlCards []*model.Card
	for _, card := range cards {
		gqlCards = append(gqlCards, convertToCard(card))
	}
	return gqlCards, nil
}

func (s *cardService) CardsByCardGroup(ctx context.Context, cardGroupID int64) ([]*model.Card, error) {
	var cards []repository.Card
	if err := s.db.WithContext(ctx).Where("cardgroup_id = ?", cardGroupID).Find(&cards).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve cards by card group ID", err)
		return nil, err
	}
	var gqlCards []*model.Card
	for _, card := range cards {
		gqlCards = append(gqlCards, convertToCard(card))
	}
	return gqlCards, nil
}

func (s *cardService) PaginatedCardsByCardGroup(ctx context.Context, cardGroupID int64, first *int, after *int64, last *int, before *int64) (*model.CardConnection, error) {
	var cards []repository.Card
	query := s.db.WithContext(ctx).Where("cardgroup_id = ?", cardGroupID)

	if after != nil {
		query = query.Where("id > ?", *after)
	}
	if before != nil {
		query = query.Where("id < ?", *before)
	}
	if first != nil {
		query = query.Order("id asc").Limit(*first)
	} else if last != nil {
		query = query.Order("id desc").Limit(*last)
	} else {
		query = query.Order("id asc").Limit(s.defaultLimit)
	}

	if err := query.Find(&cards).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve paginated cards by card group ID", err)
		return nil, err
	}

	var edges []*model.CardEdge
	var nodes []*model.Card
	for _, card := range cards {
		node := convertToCard(card)
		edges = append(edges, &model.CardEdge{
			Cursor: card.ID,
			Node:   node,
		})
		nodes = append(nodes, node)
	}

	pageInfo := &model.PageInfo{}
	if len(cards) > 0 {
		pageInfo.HasNextPage = len(cards) == s.defaultLimit
		pageInfo.HasPreviousPage = len(cards) == s.defaultLimit
		if len(edges) > 0 {
			pageInfo.StartCursor = &edges[0].Cursor
			pageInfo.EndCursor = &edges[len(edges)-1].Cursor
		}
	}

	return &model.CardConnection{
		Edges:      edges,
		Nodes:      nodes,
		PageInfo:   pageInfo,
		TotalCount: len(cards),
	}, nil
}

func (s *cardService) GetCardsByIDs(ctx context.Context, ids []int64) ([]*model.Card, error) {
	var cards []*repository.Card
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&cards).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve cards by IDs", err)
		return nil, err
	}

	var gqlCards []*model.Card
	for _, card := range cards {
		gqlCards = append(gqlCards, convertToCard(*card))
	}

	return gqlCards, nil
}
