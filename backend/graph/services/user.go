package services

import (
	"backend/graph/db"
	"backend/graph/model"
	"context"
	"fmt"
	"github.com/m-mizutani/goerr"
	"gorm.io/gorm"
	"strings"
	"time"
)

type userService struct {
	db           *gorm.DB
	defaultLimit int
}

type UserService interface {
	GetUsersByRole(ctx context.Context, roleID int64) ([]*model.User, error)
	Users(ctx context.Context) ([]*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
	CreateUser(ctx context.Context, input model.NewUser) (*model.User, error)
	UpdateUser(ctx context.Context, id int64, input model.NewUser) (*model.User, error)
	DeleteUser(ctx context.Context, id int64) (*bool, error)
	PaginatedUsersByRole(ctx context.Context, roleID int64, first *int, after *int64, last *int, before *int64) (*model.UserConnection, error)
	GetUsersByIDs(ctx context.Context, ids []int64) ([]*model.User, error)
}

func NewUserService(db *gorm.DB, defaultLimit int) UserService {
	return &userService{db: db, defaultLimit: defaultLimit}
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
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to get role by ID: %d", roleID))
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
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to get user by ID: %d", id))
	}
	return convertToUser(user), nil
}

func (s *userService) CreateUser(ctx context.Context, input model.NewUser) (*model.User, error) {
	gormUser := convertToGormUser(input)
	result := s.db.WithContext(ctx).Create(gormUser)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "unique constraint") {
			return nil, goerr.Wrap(fmt.Errorf("user already exists"), result.Error)
		}
		return nil, goerr.Wrap(result.Error, "failed to create user")
	}
	return convertToUser(*gormUser), nil
}

func (s *userService) UpdateUser(ctx context.Context, id int64, input model.NewUser) (*model.User, error) {
	var user db.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to find user for update: %d", id))
	}
	user.Name = input.Name
	user.Updated = time.Now().UTC()
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to update user")
	}
	return convertToUser(user), nil
}

func (s *userService) DeleteUser(ctx context.Context, id int64) (*bool, error) {
	success := true
	if err := s.db.WithContext(ctx).Delete(&db.User{}, id).Error; err != nil {
		success = false
		return &success, goerr.Wrap(err, fmt.Sprintf("failed to delete user: %d", id))
	}
	return &success, nil
}

func (s *userService) Users(ctx context.Context) ([]*model.User, error) {
	var users []db.User
	if err := s.db.Find(&users).Error; err != nil {
		return nil, goerr.Wrap(err, "failed to retrieve users")
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
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to fetch role: %d", roleID))
	}

	query := s.db.WithContext(ctx).Model(&role).Association("Users")

	var users []db.User
	if err := query.Find(&users); err != nil {
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to fetch users by role: %d", roleID))
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
		return nil, goerr.Wrap(err, fmt.Sprintf("failed to retrieve users by IDs: %v", ids))
	}
	var gqlUsers []*model.User
	for _, user := range users {
		gqlUsers = append(gqlUsers, convertToUser(user))
	}
	return gqlUsers, nil
}
