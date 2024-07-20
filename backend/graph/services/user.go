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
	db *gorm.DB
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
		gqlUsers = append(gqlUsers, &model.User{
			ID:      user.ID,
			Name:    user.Name,
			Created: user.Created,
			Updated: user.Updated,
		})
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
	result := s.db.WithContext(ctx).Create(&gormUser)
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

func (s *userService) DeleteUser(ctx context.Context, id int64) (bool, error) {
	if err := s.db.WithContext(ctx).Delete(&db.User{}, id).Error; err != nil {
		return false, err
	}
	return true, nil
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
