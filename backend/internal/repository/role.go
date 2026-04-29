package repository

import (
	"context"
	"errors"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

type gormRole struct {
	ID   string `gorm:"column:id;primaryKey;type:uuid"`
	Name string `gorm:"column:name"`
}

func (gormRole) TableName() string { return "roles" }

type RoleRepository interface {
	FindByName(ctx context.Context, name string) (*domain.Role, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error)
}

type roleRepo struct{ db *gorm.DB }

func NewRoleRepository(db *gorm.DB) RoleRepository { return &roleRepo{db: db} }

func (r *roleRepo) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	var row gormRole
	err := r.db.WithContext(ctx).Where("name = ?", name).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: find role by name")
	}
	return roleToDomain(row), nil
}

func (r *roleRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Role, error) {
	if len(ids) == 0 {
		return map[string]*domain.Role{}, nil
	}
	var rows []gormRole
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find roles by ids")
	}
	out := make(map[string]*domain.Role, len(rows))
	for i := range rows {
		role := roleToDomain(rows[i])
		out[role.ID] = role
	}
	return out, nil
}

func roleToDomain(g gormRole) *domain.Role {
	return &domain.Role{
		ID:   g.ID,
		Name: g.Name,
	}
}
