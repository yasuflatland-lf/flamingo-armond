package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"backend/pkg/logger"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type swipeRecordService struct {
	db           *gorm.DB
	defaultLimit int
}

func convertToGormSwipeRecord(input model.NewSwipeRecord) *repository.SwipeRecord {
	return &repository.SwipeRecord{
		UserID:    input.UserID,
		Direction: input.Direction,
		Created:   input.Created,
		Updated:   input.Updated,
	}
}

func convertToSwipeRecord(swipeRecord repository.SwipeRecord) *model.SwipeRecord {
	return &model.SwipeRecord{
		ID:        swipeRecord.ID,
		UserID:    swipeRecord.UserID,
		Direction: swipeRecord.Direction,
		Created:   swipeRecord.Created,
		Updated:   swipeRecord.Updated,
	}
}

func (s *swipeRecordService) GetSwipeRecordByID(ctx context.Context, id int64) (*model.SwipeRecord, error) {
	var swipeRecord repository.SwipeRecord
	if err := s.db.WithContext(ctx).First(&swipeRecord, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err := fmt.Errorf("swipe record not found")
			logger.Logger.ErrorContext(ctx, "Swipe record not found:", "id", id)
			return nil, err
		}
		logger.Logger.ErrorContext(ctx, "Failed to get swipe record by ID", err)
		return nil, err
	}
	return convertToSwipeRecord(swipeRecord), nil
}

func (s *swipeRecordService) CreateSwipeRecord(ctx context.Context, input model.NewSwipeRecord) (*model.SwipeRecord, error) {
	gormSwipeRecord := convertToGormSwipeRecord(input)
	result := s.db.WithContext(ctx).Create(gormSwipeRecord)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "foreign key constraint") {
			err := fmt.Errorf("invalid swipe ID or card ID")
			logger.Logger.ErrorContext(ctx, "Failed to create swipe record: invalid swipe ID or card ID", err)
			return nil, err
		}
		logger.Logger.ErrorContext(ctx, "Failed to create swipe record", result.Error)
		return nil, result.Error
	}
	return convertToSwipeRecord(*gormSwipeRecord), nil
}

func (s *swipeRecordService) UpdateSwipeRecord(ctx context.Context, id int64, input model.NewSwipeRecord) (*model.SwipeRecord, error) {
	var swipeRecord repository.SwipeRecord
	if err := s.db.WithContext(ctx).First(&swipeRecord, id).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Swipe record does not exist: ", "id", id)
		return nil, err
	}
	swipeRecord.Direction = input.Direction
	swipeRecord.Updated = time.Now()

	if err := s.db.WithContext(ctx).Save(&swipeRecord).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to save swipe record", err)
		return nil, err
	}
	return convertToSwipeRecord(swipeRecord), nil
}

func (s *swipeRecordService) DeleteSwipeRecord(ctx context.Context, id int64) (*bool, error) {
	result := s.db.WithContext(ctx).Delete(&repository.SwipeRecord{}, id)
	if result.Error != nil {
		logger.Logger.ErrorContext(ctx, "Failed to delete swipe record", result.Error)
		return nil, result.Error
	}

	success := result.RowsAffected > 0
	if !success {
		err := fmt.Errorf("record not found")
		logger.Logger.ErrorContext(ctx, "Swipe record not found for deletion", err)
		return &success, err
	}

	return &success, nil
}

func (s *swipeRecordService) SwipeRecords(ctx context.Context) ([]*model.SwipeRecord, error) {
	var swipeRecords []repository.SwipeRecord
	if err := s.db.WithContext(ctx).Find(&swipeRecords).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve swipe records", err)
		return nil, err
	}
	var gqlSwipeRecords []*model.SwipeRecord
	for _, swipeRecord := range swipeRecords {
		gqlSwipeRecords = append(gqlSwipeRecords, convertToSwipeRecord(swipeRecord))
	}
	return gqlSwipeRecords, nil
}

func (s *swipeRecordService) SwipeRecordsByUser(ctx context.Context, userID int64) ([]*model.SwipeRecord, error) {
	var swipeRecords []repository.SwipeRecord
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&swipeRecords).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve swipe records by user ID", err)
		return nil, err
	}
	var gqlSwipeRecords []*model.SwipeRecord
	for _, swipeRecord := range swipeRecords {
		gqlSwipeRecords = append(gqlSwipeRecords, convertToSwipeRecord(swipeRecord))
	}
	return gqlSwipeRecords, nil
}

func (s *swipeRecordService) PaginatedSwipeRecordsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.SwipeRecordConnection, error) {
	var swipeRecords []repository.SwipeRecord
	query := s.db.WithContext(ctx).Where("user_id = ?", userID)

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

	if err := query.Find(&swipeRecords).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve paginated swipe records by user ID", err)
		return nil, err
	}

	var edges []*model.SwipeRecordEdge
	var nodes []*model.SwipeRecord
	for _, swipeRecord := range swipeRecords {
		node := convertToSwipeRecord(swipeRecord)
		edges = append(edges, &model.SwipeRecordEdge{
			Cursor: swipeRecord.ID,
			Node:   node,
		})
		nodes = append(nodes, node)
	}

	pageInfo := &model.PageInfo{}
	if len(swipeRecords) > 0 {
		pageInfo.HasNextPage = len(swipeRecords) == s.defaultLimit
		pageInfo.HasPreviousPage = len(swipeRecords) == s.defaultLimit
		if len(edges) > 0 {
			pageInfo.StartCursor = &edges[0].Cursor
			pageInfo.EndCursor = &edges[len(edges)-1].Cursor
		}
	}

	return &model.SwipeRecordConnection{
		Edges:      edges,
		Nodes:      nodes,
		PageInfo:   pageInfo,
		TotalCount: len(swipeRecords),
	}, nil
}

func (s *swipeRecordService) GetSwipeRecordsByIDs(ctx context.Context, ids []int64) ([]*model.SwipeRecord, error) {
	var swipeRecords []*repository.SwipeRecord
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&swipeRecords).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve swipe records by IDs", err)
		return nil, err
	}

	var gqlSwipeRecords []*model.SwipeRecord
	for _, swipeRecord := range swipeRecords {
		gqlSwipeRecords = append(gqlSwipeRecords, convertToSwipeRecord(*swipeRecord))
	}

	return gqlSwipeRecords, nil
}
