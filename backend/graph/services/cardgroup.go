package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/pkg/logger"
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type cardGroupService struct {
	db           *gorm.DB
	defaultLimit int
}

func convertToGormCardGroup(input model.NewCardGroup) *repository.Cardgroup {
	return &repository.Cardgroup{
		Name:    input.Name,
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
	}
}

func convertToCardGroup(cardGroup repository.Cardgroup) *model.CardGroup {
	return &model.CardGroup{
		ID:      cardGroup.ID,
		Name:    cardGroup.Name,
		Created: cardGroup.Created,
		Updated: cardGroup.Updated,
	}
}

func (s *cardGroupService) GetCardGroupByID(ctx context.Context, id int64) (*model.CardGroup, error) {
	var cardGroup repository.Cardgroup
	if err := s.db.First(&cardGroup, id).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to get cardgroup by ID", err)
		return nil, err
	}
	return convertToCardGroup(cardGroup), nil
}

func (s *cardGroupService) CreateCardGroup(ctx context.Context, input model.NewCardGroup) (*model.CardGroup, error) {
	gormCardGroup := convertToGormCardGroup(input)
	result := s.db.WithContext(ctx).Create(&gormCardGroup)
	if result.Error != nil {
		logger.Logger.ErrorContext(ctx, "Failed to create cardgroup", result.Error)
		return nil, result.Error
	}
	return convertToCardGroup(*gormCardGroup), nil
}

func (s *cardGroupService) CardGroups(ctx context.Context) ([]*model.CardGroup, error) {
	var cardGroups []repository.Cardgroup
	if err := s.db.WithContext(ctx).Find(&cardGroups).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve cardgroups", err)
		return nil, err
	}
	var gqlCardGroups []*model.CardGroup
	for _, cardGroup := range cardGroups {
		gqlCardGroups = append(gqlCardGroups, convertToCardGroup(cardGroup))
	}
	return gqlCardGroups, nil
}

func (s *cardGroupService) UpdateCardGroup(ctx context.Context, id int64, input model.NewCardGroup) (*model.CardGroup, error) {
	var cardGroup repository.Cardgroup
	if err := s.db.WithContext(ctx).First(&cardGroup, id).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to find cardgroup for update", err)
		return nil, err
	}
	cardGroup.Name = input.Name
	cardGroup.Updated = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(&cardGroup).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to update cardgroup", err)
		return nil, err
	}
	return convertToCardGroup(cardGroup), nil
}

func (s *cardGroupService) DeleteCardGroup(ctx context.Context, id int64) (*bool, error) {
	success := false
	result := s.db.WithContext(ctx).Delete(&repository.Cardgroup{}, id)
	if result.Error != nil {
		logger.Logger.ErrorContext(ctx, "Failed to delete cardgroup", result.Error)
		return &success, result.Error
	}
	if result.RowsAffected == 0 {
		err := fmt.Errorf("record not found")
		logger.Logger.ErrorContext(ctx, "Cardgroup not found for deletion", err)
		return &success, err
	}
	success = true
	return &success, nil
}

func (s *cardGroupService) AddUserToCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error) {
	var user repository.User
	var cardGroup repository.Cardgroup
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to find user for adding to cardgroup", err)
		return nil, err
	}
	if err := s.db.WithContext(ctx).First(&cardGroup, cardGroupID).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to find cardgroup for adding user", err)
		return nil, err
	}
	if err := s.db.Model(&cardGroup).Association("Users").Append(&user); err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to add user to cardgroup", err)
		return nil, err
	}
	return convertToCardGroup(cardGroup), nil
}

func (s *cardGroupService) RemoveUserFromCardGroup(ctx context.Context, userID int64, cardGroupID int64) (*model.CardGroup, error) {
	var user repository.User
	var cardGroup repository.Cardgroup
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to find user for removing from cardgroup", err)
		return nil, err
	}
	if err := s.db.WithContext(ctx).First(&cardGroup, cardGroupID).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to find cardgroup for removing user", err)
		return nil, err
	}
	if err := s.db.Model(&cardGroup).Association("Users").Delete(&user); err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to remove user from cardgroup", err)
		return nil, err
	}
	return convertToCardGroup(cardGroup), nil
}

func (s *cardGroupService) GetCardGroupsByUser(ctx context.Context, userID int64) ([]*model.CardGroup, error) {
	var user repository.User
	if err := s.db.WithContext(ctx).Preload("CardGroups").First(&user, userID).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to get cardgroups by user ID", err)
		return nil, err
	}
	var gqlCardGroups []*model.CardGroup
	for _, group := range user.CardGroups {
		gqlCardGroups = append(gqlCardGroups, convertToCardGroup(group))
	}
	return gqlCardGroups, nil
}

func (s *cardGroupService) PaginatedCardGroupsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.CardGroupConnection, error) {
	var user repository.User
	var cardGroups []repository.Cardgroup

	// Fetch the user and preload the cardgroups with pagination conditions
	query := s.db.WithContext(ctx).Model(&user).Where("id = ?", userID).Preload("CardGroups", func(db *gorm.DB) *gorm.DB {
		if after != nil {
			db = db.Where("cardgroups.id > ?", *after)
		}
		if before != nil {
			db = db.Where("cardgroups.id < ?", *before)
		}
		if first != nil {
			db = db.Order("cardgroups.id asc").Limit(*first)
		} else if last != nil {
			db = db.Order("cardgroups.id desc").Limit(*last)
		} else {
			db = db.Order("cardgroups.id asc").Limit(s.defaultLimit)
		}
		return db
	})

	if err := query.Find(&user).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to get paginated cardgroups by user", err)
		return nil, fmt.Errorf("error fetching paginated cardgroups by user: %+v", err)
	}

	cardGroups = user.CardGroups

	var edges []*model.CardGroupEdge
	var nodes []*model.CardGroup
	for _, cardGroup := range cardGroups {
		node := convertToCardGroup(cardGroup)
		edges = append(edges, &model.CardGroupEdge{
			Cursor: cardGroup.ID,
			Node:   node,
		})
		nodes = append(nodes, node)
	}

	pageInfo := &model.PageInfo{}
	if len(cardGroups) > 0 {
		pageInfo.HasNextPage = len(cardGroups) == s.defaultLimit
		pageInfo.HasPreviousPage = len(cardGroups) == s.defaultLimit
		if len(edges) > 0 {
			pageInfo.StartCursor = &edges[0].Cursor
			pageInfo.EndCursor = &edges[len(edges)-1].Cursor
		}
	}

	return &model.CardGroupConnection{
		Edges:      edges,
		Nodes:      nodes,
		PageInfo:   pageInfo,
		TotalCount: len(cardGroups),
	}, nil
}

func (s *cardGroupService) GetCardGroupsByIDs(ctx context.Context, ids []int64) ([]*model.CardGroup, error) {
	var cardGroups []*repository.Cardgroup
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&cardGroups).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve cardgroups by IDs", err)
		return nil, err
	}

	var gqlCardGroups []*model.CardGroup
	for _, cardGroup := range cardGroups {
		gqlCardGroups = append(gqlCardGroups, convertToCardGroup(*cardGroup))
	}

	return gqlCardGroups, nil
}
