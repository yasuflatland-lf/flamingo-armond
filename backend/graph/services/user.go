package services

import (
	"backend/graph/db"
	"backend/graph/model"
	"backend/pkg/logger"
	"context"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

type userService struct {
	db           *gorm.DB
	defaultLimit int
}

func convertToGormUser(input model.NewUser) *db.User {
	return &db.User{
		Name:    input.Name,
		Created: time.Now().UTC(),
		Updated: time.Now().UTC(),
	}
}

func convertToUser(user db.User) *model.User {
	return &model.User{
		ID:      user.ID,
		Name:    user.Name,
		Created: user.Created,
		Updated: user.Updated,
	}
}

func (s *userService) GetUsersByRole(ctx context.Context, roleID int64) ([]*model.User, error) {
	var role db.Role
	if err := s.db.WithContext(ctx).Preload("Users").First(&role, roleID).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to get role by ID", err)
		return nil, err
	}
	var gqlUsers []*model.User
	for _, user := range role.Users {
		gqlUsers = append(gqlUsers, convertToUser(user))
	}
	return gqlUsers, nil
}

func (s *userService) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	var user db.User
	if err := s.db.First(&user, id).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to get user by ID", err)
		return nil, err
	}
	return convertToUser(user), nil
}

func (s *userService) CreateUser(ctx context.Context, input model.NewUser) (*model.User, error) {
	gormUser := convertToGormUser(input)
	result := s.db.WithContext(ctx).Create(gormUser)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "unique constraint") {
			err := fmt.Errorf("user already exists")
			logger.Logger.ErrorContext(ctx, "Failed to create user: user already exists", err)
			return nil, err
		}
		logger.Logger.ErrorContext(ctx, "Failed to create user", result.Error)
		return nil, result.Error
	}
	return convertToUser(*gormUser), nil
}

func (s *userService) UpdateUser(ctx context.Context, id int64, input model.NewUser) (*model.User, error) {
	var user db.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to find user for update", err)
		return nil, err
	}
	user.Name = input.Name
	user.Updated = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to update user", err)
		return nil, err
	}
	return convertToUser(user), nil
}

func (s *userService) DeleteUser(ctx context.Context, id int64) (*bool, error) {
	success := true
	if err := s.db.WithContext(ctx).Delete(&db.User{}, id).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to delete user", err)
		success = false
		return &success, err
	}
	return &success, nil
}

func (s *userService) Users(ctx context.Context) ([]*model.User, error) {
	var users []db.User
	if err := s.db.Find(&users).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve users", err)
		return nil, err
	}
	var gqlUsers []*model.User
	for _, user := range users {
		gqlUsers = append(gqlUsers, convertToUser(user))
	}
	return gqlUsers, nil
}

func (s *userService) PaginatedUsersByRole(ctx context.Context, roleID int64, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error) {
	var role db.Role
	if err := s.db.WithContext(ctx).Preload("Users").First(&role, roleID).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Error fetching role", err)
		return nil, fmt.Errorf("error fetching role: %+v", err)
	}

	query := s.db.WithContext(ctx).Model(&role).Association("Users")

	var users []db.User
	if err := query.Find(&users); err != nil {
		logger.Logger.ErrorContext(ctx, "Error fetching users by role", err)
		return nil, fmt.Errorf("error fetching users by role: %+v", err)
	}

	// Implement pagination logic
	start := 0
	end := len(users)
	if after != nil {
		for i, user := range users {
			if user.ID > *after {
				start = i + 1
				break
			}
		}
	}
	if before != nil {
		for i, user := range users {
			if user.ID >= *before {
				end = i
				break
			}
		}
	}
	if first != nil {
		if start+*first < end {
			end = start + *first
		}
	}
	if last != nil {
		if end-*last > start {
			start = end - *last
		}
	}

	paginatedUsers := users[start:end]

	var edges []*model.UserEdge
	var nodes []*model.User
	for _, user := range paginatedUsers {
		node := convertToUser(user)
		edges = append(edges, &model.UserEdge{
			Cursor: user.ID,
			Node:   node,
		})
		nodes = append(nodes, node)
	}

	pageInfo := &model.PageInfo{}
	if len(users) > 0 {
		pageInfo.HasNextPage = end < len(users)
		pageInfo.HasPreviousPage = start > 0
		if len(edges) > 0 {
			pageInfo.StartCursor = &edges[0].Cursor
			pageInfo.EndCursor = &edges[len(edges)-1].Cursor
		}
	}

	return &model.UserConnection{
		Edges:      edges,
		Nodes:      nodes,
		PageInfo:   pageInfo,
		TotalCount: len(users),
	}, nil
}

func (s *userService) GetUsersByIDs(ctx context.Context, ids []int64) ([]*model.User, error) {
	var users []db.User
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		logger.Logger.ErrorContext(ctx, "Failed to retrieve users by IDs", err)
		return nil, err
	}
	var gqlUsers []*model.User
	for _, user := range users {
		gqlUsers = append(gqlUsers, convertToUser(user))
	}
	return gqlUsers, nil
}
