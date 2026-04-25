package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"backend/internal/domain"
)

// gormProfile is the row mapping for public.profiles. Package-private so
// callers cannot bypass the domain conversion.
type gormProfile struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid"`
	DisplayName *string   `gorm:"column:display_name"`
	Bio         *string   `gorm:"column:bio"`
	AvatarURL   *string   `gorm:"column:avatar_url"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (gormProfile) TableName() string { return "profiles" }

// ErrNotFound is returned when a profile lookup or update targets a row that
// does not exist.
var ErrNotFound = errors.New("repository: profile not found")

// ProfileUpdate carries patch fields. nil means "leave untouched"; a non-nil
// pointer to "" is a request to clear the column.
type ProfileUpdate struct {
	DisplayName *string
	Bio         *string
	AvatarURL   *string
}

type ProfileRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Profile, error)
	Update(ctx context.Context, id string, patch ProfileUpdate) (*domain.Profile, error)
}

type profileRepo struct{ db *gorm.DB }

func NewProfileRepository(db *gorm.DB) ProfileRepository { return &profileRepo{db: db} }

func (r *profileRepo) FindByID(ctx context.Context, id string) (*domain.Profile, error) {
	var row gormProfile
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return toDomain(row), nil
}

func (r *profileRepo) Update(ctx context.Context, id string, patch ProfileUpdate) (*domain.Profile, error) {
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

	res := r.db.WithContext(ctx).Model(&gormProfile{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	// Re-fetch so callers see the trigger-refreshed updated_at.
	return r.FindByID(ctx, id)
}

func toDomain(g gormProfile) *domain.Profile {
	return &domain.Profile{
		ID:          g.ID,
		DisplayName: g.DisplayName,
		Bio:         g.Bio,
		AvatarURL:   g.AvatarURL,
		CreatedAt:   g.CreatedAt,
		UpdatedAt:   g.UpdatedAt,
	}
}
