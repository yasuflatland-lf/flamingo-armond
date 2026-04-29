package repository

import (
	"context"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"
)

type gormUserRole struct {
	UserID string `gorm:"column:user_id;primaryKey;type:uuid"`
	RoleID string `gorm:"column:role_id;primaryKey;type:uuid"`
}

func (gormUserRole) TableName() string { return "user_roles" }

type UserRoleRepository interface {
	HasRole(ctx context.Context, userID, roleName string) (bool, error)
}

type userRoleRepo struct{ db *gorm.DB }

func NewUserRoleRepository(db *gorm.DB) UserRoleRepository {
	return &userRoleRepo{db: db}
}

func (r *userRoleRepo) HasRole(ctx context.Context, userID, roleName string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id = ? AND roles.name = ?", userID, roleName).
		Count(&count).Error
	if err != nil {
		return false, eris.Wrap(err, "repository: check user role")
	}
	return count > 0, nil
}
