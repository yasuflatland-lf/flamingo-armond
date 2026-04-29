package repository

import (
	"context"
	"errors"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// gormCardgroup is the row mapping for public.cardgroups. Package-private so
// callers cannot bypass the domain conversion.
type gormCardgroup struct {
	ID        string    `gorm:"column:id;primaryKey;type:uuid"`
	OwnerID   string    `gorm:"column:owner_id"`
	Name      string    `gorm:"column:name"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (gormCardgroup) TableName() string { return "cardgroups" }

// CardgroupUpdate carries patch fields. nil means "leave untouched".
type CardgroupUpdate struct {
	Name *string
}

// CardgroupRepository provides persistence operations for the Cardgroup
// aggregate.
type CardgroupRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
	FindByOwner(ctx context.Context, ownerID string) ([]*domain.Cardgroup, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Cardgroup, error)
	Create(ctx context.Context, cg *domain.Cardgroup) error
	Update(ctx context.Context, id string, patch CardgroupUpdate) (*domain.Cardgroup, error)
	Delete(ctx context.Context, id string) error
}

type cardgroupRepo struct{ db *gorm.DB }

// NewCardgroupRepository returns a GORM-backed CardgroupRepository.
func NewCardgroupRepository(db *gorm.DB) CardgroupRepository { return &cardgroupRepo{db: db} }

// FindByID returns the cardgroup with the given id, or ErrNotFound.
func (r *cardgroupRepo) FindByID(ctx context.Context, id string) (*domain.Cardgroup, error) {
	var row gormCardgroup
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: find cardgroup by id")
	}
	return cardgroupToDomain(row), nil
}

// FindByOwner returns all cardgroups owned by ownerID, ordered by updated_at
// DESC. Returns an empty slice when none are found.
func (r *cardgroupRepo) FindByOwner(ctx context.Context, ownerID string) ([]*domain.Cardgroup, error) {
	var rows []gormCardgroup
	if err := r.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Order("updated_at DESC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find cardgroups by owner")
	}
	out := make([]*domain.Cardgroup, len(rows))
	for i := range rows {
		out[i] = cardgroupToDomain(rows[i])
	}
	return out, nil
}

// FindByIDs returns a map of id → Cardgroup for all found ids. IDs that do not
// exist are simply absent from the map. Short-circuits on an empty input slice
// to avoid an unfiltered table scan.
func (r *cardgroupRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Cardgroup, error) {
	if len(ids) == 0 {
		return map[string]*domain.Cardgroup{}, nil
	}
	var rows []gormCardgroup
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find cardgroups by ids")
	}
	out := make(map[string]*domain.Cardgroup, len(rows))
	for i := range rows {
		cg := cardgroupToDomain(rows[i])
		out[cg.ID] = cg
	}
	return out, nil
}

// Create inserts a new cardgroup row. The caller is responsible for pre-filling
// cg.ID (uuid v7) and both timestamps.
func (r *cardgroupRepo) Create(ctx context.Context, cg *domain.Cardgroup) error {
	row := gormCardgroup{
		ID:        cg.ID,
		OwnerID:   cg.OwnerID,
		Name:      cg.Name,
		CreatedAt: cg.CreatedAt,
		UpdatedAt: cg.UpdatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return eris.Wrap(err, "repository: create cardgroup")
	}
	return nil
}

// Update applies a partial patch to the cardgroup identified by id. If the
// patch is empty (all fields nil) the current row is returned without touching
// the database. Returns ErrNotFound when no row matches.
func (r *cardgroupRepo) Update(ctx context.Context, id string, patch CardgroupUpdate) (*domain.Cardgroup, error) {
	updates := map[string]any{}
	if patch.Name != nil {
		updates["name"] = *patch.Name
	}
	if len(updates) == 0 {
		// No-op patch: return current value rather than touching DB.
		return r.FindByID(ctx, id)
	}

	res := r.db.WithContext(ctx).Model(&gormCardgroup{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, eris.Wrap(res.Error, "repository: update cardgroup")
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	// Re-fetch so callers see the trigger-refreshed updated_at.
	return r.FindByID(ctx, id)
}

// Delete removes the cardgroup identified by id. Returns ErrNotFound when no
// row matches.
func (r *cardgroupRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormCardgroup{})
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: delete cardgroup")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func cardgroupToDomain(g gormCardgroup) *domain.Cardgroup {
	return &domain.Cardgroup{
		ID:        g.ID,
		OwnerID:   g.OwnerID,
		Name:      g.Name,
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}
}
