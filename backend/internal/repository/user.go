package repository

import (
	"context"
	"errors"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// gormUser is the row mapping for public.users. Package-private so
// callers cannot bypass the domain conversion.
type gormUser struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid"`
	DisplayName *string   `gorm:"column:display_name"`
	Bio         *string   `gorm:"column:bio"`
	AvatarURL   *string   `gorm:"column:avatar_url"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (gormUser) TableName() string { return "users" }

// ErrNotFound is returned when a lookup or update targets a row that does not
// exist.
var ErrNotFound = errors.New("repository: not found")

// UserUpdate carries patch fields. nil means "leave untouched"; a non-nil
// pointer to "" is a request to clear the column.
type UserUpdate struct {
	DisplayName *string
	Bio         *string
	AvatarURL   *string
}

type UserRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.User, error)
	Update(ctx context.Context, id string, patch UserUpdate) (*domain.User, error)
}

type userRepo struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) UserRepository { return &userRepo{db: db} }

func (r *userRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	var row gormUser
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: find user by id")
	}
	return userToDomain(row), nil
}

func (r *userRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.User, error) {
	// GORM turns WHERE id IN () into an unfiltered scan, so short-circuit empty input.
	if len(ids) == 0 {
		return map[string]*domain.User{}, nil
	}
	var rows []gormUser
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find users by ids")
	}
	out := make(map[string]*domain.User, len(rows))
	for i := range rows {
		u := userToDomain(rows[i])
		out[u.ID] = u
	}
	return out, nil
}

func (r *userRepo) Update(ctx context.Context, id string, patch UserUpdate) (*domain.User, error) {
	updates := map[string]any{}
	if patch.DisplayName != nil {
		updates["display_name"] = *patch.DisplayName
	}
	if patch.Bio != nil {
		updates["bio"] = *patch.Bio
	}
	if patch.AvatarURL != nil {
		updates["avatar_url"] = *patch.AvatarURL
	}
	if len(updates) == 0 {
		// No-op patch: return current value rather than touching DB.
		return r.FindByID(ctx, id)
	}

	res := r.db.WithContext(ctx).Model(&gormUser{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, eris.Wrap(res.Error, "repository: update user")
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	// Re-fetch so callers see the trigger-refreshed updated_at.
	return r.FindByID(ctx, id)
}

func userToDomain(g gormUser) *domain.User {
	return &domain.User{
		ID:          g.ID,
		DisplayName: g.DisplayName,
		Bio:         g.Bio,
		AvatarURL:   g.AvatarURL,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}
}
