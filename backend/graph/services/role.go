package services

import (
	"backend/graph/db"
	"backend/graph/model"
	"context"
	"fmt"
	"gorm.io/gorm"
)

type roleService struct {
	db *gorm.DB
}

func convertToRole(role db.Role) *model.Role {
	return &model.Role{
		ID:   role.ID,
		Name: role.Name,
	}
}

func convertToGormRole(input model.NewRole) db.Role {
	return db.Role{
		Name: input.Name,
	}
}

func (s *roleService) GetRoleByUserID(ctx context.Context, userID int64) (*model.Role, error) {
	var user db.User
	if err := s.db.WithContext(ctx).Preload("Roles").First(&user, userID).Error; err != nil {
		return nil, err
	}
	if len(user.Roles) == 0 {
		return nil, fmt.Errorf("user has no role")
	}
	role := user.Roles[0] // Assuming a user has only one role
	return convertToRole(role), nil
}

func (s *roleService) GetRoleByID(ctx context.Context, id int64) (*model.Role, error) {
	var role db.Role
	if err := s.db.WithContext(ctx).First(&role, id).Error; err != nil {
		return nil, err
	}
	return convertToRole(role), nil
}

func (s *roleService) CreateRole(ctx context.Context, input model.NewRole) (*model.Role, error) {
	gormRole := convertToGormRole(input)
	result := s.db.WithContext(ctx).Create(&gormRole)
	if result.Error != nil {
		return nil, result.Error
	}
	return convertToRole(gormRole), nil
}

func (s *roleService) UpdateRole(ctx context.Context, id int64, input model.NewRole) (*model.Role, error) {
	var role db.Role
	if err := s.db.WithContext(ctx).First(&role, id).Error; err != nil {
		return nil, err
	}
	role.Name = input.Name
	if err := s.db.WithContext(ctx).Save(&role).Error; err != nil {
		return nil, err
	}
	return convertToRole(role), nil
}

func (s *roleService) DeleteRole(ctx context.Context, id int64) (bool, error) {
	if err := s.db.WithContext(ctx).Delete(&db.Role{}, id).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (s *roleService) AssignRoleToUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	var user db.User
	var role db.Role
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).First(&role, roleID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Append(&role); err != nil {
		return nil, err
	}
	return &model.User{
		ID:      user.ID,
		Name:    user.Name,
		Created: user.Created,
		Updated: user.Updated,
	}, nil
}

func (s *roleService) RemoveRoleFromUser(ctx context.Context, userID int64, roleID int64) (*model.User, error) {
	var user db.User
	var role db.Role
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).First(&role, roleID).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Model(&user).Association("Roles").Delete(&role); err != nil {
		return nil, err
	}
	return &model.User{
		ID:      user.ID,
		Name:    user.Name,
		Created: user.Created,
		Updated: user.Updated,
	}, nil
}

func (s *roleService) Roles(ctx context.Context) ([]*model.Role, error) {
	var roles []db.Role
	if err := s.db.WithContext(ctx).Find(&roles).Error; err != nil {
		return nil, err
	}
	var gqlRoles []*model.Role
	for _, role := range roles {
		gqlRoles = append(gqlRoles, &model.Role{
			ID:      role.ID,
			Name:    role.Name,
			Created: role.Created,
			Updated: role.Updated,
		})
	}
	return gqlRoles, nil
}
