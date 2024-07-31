package services

import (
	"backend/graph/db"
	"backend/graph/model"
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
		Created: time.Now(),
		Updated: time.Now(),
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
		return nil, err
	}
	return convertToUser(user), nil
}

func (s *userService) CreateUser(ctx context.Context, input model.NewUser) (*model.User, error) {
	gormUser := convertToGormUser(input)
	result := s.db.WithContext(ctx).Create(gormUser)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "unique constraint") {
			return nil, fmt.Errorf("user already exists")
		}
		return nil, result.Error
	}
	return convertToUser(*gormUser), nil
}

func (s *userService) UpdateUser(ctx context.Context, id int64, input model.NewUser) (*model.User, error) {
	var user db.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, err
	}
	user.Name = input.Name
	user.Updated = time.Now()
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return nil, err
	}
	return convertToUser(user), nil
}

func (s *userService) DeleteUser(ctx context.Context, id int64) (*bool, error) {
	success := true
	if err := s.db.WithContext(ctx).Delete(&db.User{}, id).Error; err != nil {
		success = false
		return &success, err
	}
	return &success, nil
}

func (s *userService) Users(ctx context.Context) ([]*model.User, error) {
	var users []db.User
	if err := s.db.Find(&users).Error; err != nil {
		return nil, err
	}
	var gqlUsers []*model.User
	for _, user := range users {
		gqlUsers = append(gqlUsers, convertToUser(user))
	}
	return gqlUsers, nil
}

func (s *userService) PaginatedUsers(ctx context.Context, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error) {
	var users []db.User
	query := s.db.WithContext(ctx)

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

	if err := query.Find(&users).Error; err != nil {
		return nil, err
	}

	var edges []*model.UserEdge
	var nodes []*model.User
	for _, user := range users {
		node := convertToUser(user)
		edges = append(edges, &model.UserEdge{
			Cursor: user.ID,
			Node:   node,
		})
		nodes = append(nodes, node)
	}

	pageInfo := &model.PageInfo{}
	if len(users) > 0 {
		pageInfo.HasNextPage = len(users) == s.defaultLimit
		pageInfo.HasPreviousPage = len(users) == s.defaultLimit
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

func (s *userService) PaginatedUsersByRole(ctx context.Context, roleID int64, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error) {
	var users []db.User
	query := s.db.WithContext(ctx).Model(&db.Role{ID: roleID})

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

	if err := query.Association("Users").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("error %+v", err())
	}

	var edges []*model.UserEdge
	var nodes []*model.User
	for _, user := range users {
		node := convertToUser(user)
		edges = append(edges, &model.UserEdge{
			Cursor: user.ID,
			Node:   node,
		})
		nodes = append(nodes, node)
	}

	pageInfo := &model.PageInfo{}
	if len(users) > 0 {
		pageInfo.HasNextPage = len(users) == s.defaultLimit
		pageInfo.HasPreviousPage = len(users) == s.defaultLimit
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
	var users []*model.User
	if err := s.db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
