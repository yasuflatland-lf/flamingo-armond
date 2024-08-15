package services

import (
	repository "backend/graph/db"
	"backend/graph/model"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/m-mizutani/goerr"
	"gorm.io/gorm"
)

type swipeRecordService struct {
	db           *gorm.DB
	defaultLimit int
}

const (
	KNOWN    = "known"
	DONTKNOW = "dontknow"
	MAYBE    = "maybe"
)

type SwipeRecordService interface {
	GetSwipeRecordByID(ctx context.Context, id int64) (*model.SwipeRecord, error)
	CreateSwipeRecord(ctx context.Context, input model.NewSwipeRecord) (*model.SwipeRecord, error)
	UpdateSwipeRecord(ctx context.Context, id int64, input model.NewSwipeRecord) (*model.SwipeRecord, error)
	DeleteSwipeRecord(ctx context.Context, id int64) (*bool, error)
	SwipeRecords(ctx context.Context) ([]*model.SwipeRecord, error)
	SwipeRecordsByUser(ctx context.Context, userID int64) ([]*model.SwipeRecord, error)
	PaginatedSwipeRecordsByUser(ctx context.Context, userID int64, first *int, after *int64, last *int, before *int64) (*model.SwipeRecordConnection, error)
	GetSwipeRecordsByIDs(ctx context.Context, ids []int64) ([]*model.SwipeRecord, error)
	GetSwipeRecordsByUserAndOrder(ctx context.Context, userID int64, order string, limit int) ([]repository.SwipeRecord, error)
}

func NewSwipeRecordService(db *gorm.DB, defaultLimit int) SwipeRecordService {
	return &swipeRecordService{db: db, defaultLimit: defaultLimit}
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
			return nil, goerr.Wrap(fmt.Errorf("swipe record not found: id=%d", id), err)
		}
		return nil, goerr.Wrap(err, "failed to get swipe record by ID")
	}
	return convertToSwipeRecord(swipeRecord), nil
}

func (s *swipeRecordService) CreateSwipeRecord(ctx context.Context, input model.NewSwipeRecord) (*model.SwipeRecord, error) {
	gormSwipeRecord := convertToGormSwipeRecord(input)
	result := s.db.WithContext(ctx).Create(gormSwipeRecord)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "foreign key constraint") {
			return nil, goerr.Wrap(fmt.Errorf("invalid swipe ID or card ID"), result.Error)
		}
		return nil, goerr.Wrap(result.Error, "failed to create swipe record")
	}
	return convertToSwipeRecord(*gormSwipeRecord), nil
}

func (s *swipeRecordService) UpdateSwipeRecord(ctx context.Context, id int64, input model.NewSwipeRecord) (*model.SwipeRecord, error) {
	var swipeRecord repository.SwipeRecord
	if err := s.db.WithContext(ctx).First(&swipeRecord, id).Error; err != nil {
		return nil, goerr.Wrap(fmt.Errorf("swipe record does not exist: id=%d", id), err)
	}
	swipeRecord.Direction = input.Direction
	swipeRecord.Updated = time.Now().UTC()

	if err := s.db.WithContext(ctx).Save(&swipeRecord).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to update swipe record")
	}
	return convertToSwipeRecord(swipeRecord), nil
}

func (s *swipeRecordService) DeleteSwipeRecord(ctx context.Context, id int64) (*bool, error) {
	result := s.db.WithContext(ctx).Delete(&repository.SwipeRecord{}, id)
	if result.Error != nil {
		return nil, goerr.Wrap(result.Error, "failed to delete swipe record")
	}

	success := result.RowsAffected > 0
	if !success {
		return &success, goerr.Wrap(fmt.Errorf("record not found: id=%d", id))
	}

	return &success, nil
}

func (s *swipeRecordService) SwipeRecords(ctx context.Context) ([]*model.SwipeRecord, error) {
	var swipeRecords []repository.SwipeRecord
	if err := s.db.WithContext(ctx).Find(&swipeRecords).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to retrieve swipe records")
	}
	var gqlSwipeRecords []*model.SwipeRecord
	for _, swipeRecord := range swipeRecords {
		gqlSwipeRecords = append(gqlSwipeRecords, convertToSwipeRecord(swipeRecord))
	}
	return gqlSwipeRecords, nil
}

func (s *swipeRecordService) SwipeRecordsByUser(ctx context.Context, userID int64) ([]*model.SwipeRecord, error) {
	if userID <= 0 {
		return nil, goerr.Wrap(fmt.Errorf("user ID must be larger than 0. It's %d", userID))
	}

	var swipeRecords []repository.SwipeRecord
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&swipeRecords).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to retrieve swipe records by user ID: %d", userID))
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
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to retrieve paginated swipe records by user ID: %d", userID))
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
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to retrieve swipe records by IDs: %v", ids))
	}

	var gqlSwipeRecords []*model.SwipeRecord
	for _, swipeRecord := range swipeRecords {
		gqlSwipeRecords = append(gqlSwipeRecords, convertToSwipeRecord(*swipeRecord))
	}

	return gqlSwipeRecords, nil
}

func (s *swipeRecordService) GetSwipeRecordsByUserAndOrder(ctx context.Context, userID int64, order string, limit int) ([]repository.SwipeRecord, error) {

	orderClause := fmt.Sprintf("updated %s", order)

	var swipeRecords []repository.SwipeRecord

	query := s.db.WithContext(ctx).Where("user_id = ?", userID).Limit(limit).Order(orderClause)

	if err := query.Find(&swipeRecords).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Sprintf("User id: %d", userID))
	}

	return swipeRecords, nil
}
