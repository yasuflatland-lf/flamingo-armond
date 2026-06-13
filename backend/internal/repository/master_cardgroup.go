package repository

import (
	"context"
	"errors"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// gormMasterCardgroup is the row mapping for public.master_cardgroups. Package-
// private so callers cannot bypass the domain conversion. Do NOT embed this
// struct in any outer scan target — it declares TableName(), which would hijack
// the embedding struct's scan target.
type gormMasterCardgroup struct {
	ID               string    `gorm:"column:id;primaryKey;type:uuid"`
	Name             string    `gorm:"column:name"`
	Description      *string   `gorm:"column:description"`
	Language         *string   `gorm:"column:language"`
	Level            *string   `gorm:"column:level"`
	Category         *string   `gorm:"column:category"`
	CoverImageURL    *string   `gorm:"column:cover_image_url"`
	Source           *string   `gorm:"column:source"`
	Version          int       `gorm:"column:version"`
	Status           string    `gorm:"column:status"`
	IsDefaultStarter bool      `gorm:"column:is_default_starter"`
	SortOrder        int       `gorm:"column:sort_order"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (gormMasterCardgroup) TableName() string { return "master_cardgroups" }

// MasterCardgroupUpdate carries patch fields. nil means "leave untouched".
type MasterCardgroupUpdate struct {
	Name             *string
	Description      *string
	Language         *string
	Level            *string
	Category         *string
	CoverImageURL    *string
	Source           *string
	Version          *int
	SortOrder        *int
	Status           *string
	IsDefaultStarter *bool
}

// MasterCardgroupRepository provides persistence operations for the
// MasterCardgroup aggregate — the admin-managed catalog template that is never
// directly owned by an end user.
type MasterCardgroupRepository interface {
	FindByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	EnsureByName(ctx context.Context, name string) (*domain.MasterCardgroup, error)
	Create(ctx context.Context, m *domain.MasterCardgroup) error
	Update(ctx context.Context, id string, patch MasterCardgroupUpdate) (*domain.MasterCardgroup, error)
	Delete(ctx context.Context, id string) error
	ListDefaultStarters(ctx context.Context) ([]*domain.MasterCardgroup, error)
}

type masterCardgroupRepo struct{ db *gorm.DB }

// NewMasterCardgroupRepository returns a GORM-backed MasterCardgroupRepository.
func NewMasterCardgroupRepository(db *gorm.DB) MasterCardgroupRepository {
	return &masterCardgroupRepo{db: db}
}

// FindByID returns the master cardgroup with the given id, or ErrNotFound.
func (r *masterCardgroupRepo) FindByID(ctx context.Context, id string) (*domain.MasterCardgroup, error) {
	var row gormMasterCardgroup
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: master cardgroup: find by id")
	}
	return masterCardgroupToDomain(row), nil
}

// EnsureByName returns the existing master cardgroup with the given name or
// creates it when absent. The master_cardgroups table has no UNIQUE(name)
// constraint, so two concurrent callers could otherwise insert duplicate name
// rows. This method takes a transaction-scoped advisory lock keyed on the
// literal 'master' namespace plus the name to serialize lookup-then-insert
// without adding a DB-level constraint. Master cardgroups have no owner.
func (r *masterCardgroupRepo) EnsureByName(ctx context.Context, name string) (*domain.MasterCardgroup, error) {
	var out *domain.MasterCardgroup
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?), hashtext(?))", "master", name).Error; err != nil {
			return eris.Wrap(err, "repository: master cardgroup: ensure by name: advisory lock")
		}

		var row gormMasterCardgroup
		err := tx.Where("name = ?", name).Take(&row).Error
		if err == nil {
			out = masterCardgroupToDomain(row)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return eris.Wrap(err, "repository: master cardgroup: ensure by name: lookup")
		}

		id, err := domain.NewID()
		if err != nil {
			return eris.Wrap(err, "repository: master cardgroup: ensure by name: id")
		}
		now := time.Now().UTC()
		row = gormMasterCardgroup{
			ID:               id,
			Name:             name,
			Version:          1,
			Status:           string(domain.MasterStatusDraft),
			IsDefaultStarter: false,
			SortOrder:        0,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return eris.Wrap(err, "repository: master cardgroup: ensure by name: create")
		}
		out = masterCardgroupToDomain(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Create inserts a new master cardgroup row. The caller is responsible for
// pre-filling m.ID (uuid v7 via domain.NewID) and both timestamps.
func (r *masterCardgroupRepo) Create(ctx context.Context, m *domain.MasterCardgroup) error {
	row := masterCardgroupFromDomain(m)
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return eris.Wrap(err, "repository: master cardgroup: create")
	}
	return nil
}

// Update applies a partial patch to the master cardgroup identified by id. If
// the patch is empty (all fields nil) the current row is returned without
// touching the database. Returns ErrNotFound when no row matches.
func (r *masterCardgroupRepo) Update(ctx context.Context, id string, patch MasterCardgroupUpdate) (*domain.MasterCardgroup, error) {
	updates := map[string]any{}
	if patch.Name != nil {
		updates["name"] = *patch.Name
	}
	if patch.Description != nil {
		updates["description"] = *patch.Description
	}
	if patch.Language != nil {
		updates["language"] = *patch.Language
	}
	if patch.Level != nil {
		updates["level"] = *patch.Level
	}
	if patch.Category != nil {
		updates["category"] = *patch.Category
	}
	if patch.CoverImageURL != nil {
		updates["cover_image_url"] = *patch.CoverImageURL
	}
	if patch.Source != nil {
		updates["source"] = *patch.Source
	}
	if patch.Version != nil {
		updates["version"] = *patch.Version
	}
	if patch.SortOrder != nil {
		updates["sort_order"] = *patch.SortOrder
	}
	if patch.Status != nil {
		updates["status"] = *patch.Status
	}
	if patch.IsDefaultStarter != nil {
		updates["is_default_starter"] = *patch.IsDefaultStarter
	}
	if len(updates) == 0 {
		// No-op patch: return current value rather than touching DB.
		return r.FindByID(ctx, id)
	}

	res := r.db.WithContext(ctx).Model(&gormMasterCardgroup{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, eris.Wrap(res.Error, "repository: master cardgroup: update")
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	// Re-fetch so callers see the trigger-refreshed updated_at.
	return r.FindByID(ctx, id)
}

// Delete removes the master cardgroup identified by id. Returns ErrNotFound
// when no row matches.
func (r *masterCardgroupRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormMasterCardgroup{})
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: master cardgroup: delete")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListDefaultStarters returns all published master cardgroups flagged as
// default starters, ordered by (sort_order, id) so the starter set is
// deterministic. Returns an empty slice when none are found.
func (r *masterCardgroupRepo) ListDefaultStarters(ctx context.Context) ([]*domain.MasterCardgroup, error) {
	var rows []gormMasterCardgroup
	if err := r.db.WithContext(ctx).
		Where("status = ? AND is_default_starter", string(domain.MasterStatusPublished)).
		Order("sort_order, id").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: master cardgroup: list default starters")
	}
	out := make([]*domain.MasterCardgroup, len(rows))
	for i := range rows {
		out[i] = masterCardgroupToDomain(rows[i])
	}
	return out, nil
}

func masterCardgroupToDomain(g gormMasterCardgroup) *domain.MasterCardgroup {
	return &domain.MasterCardgroup{
		ID:               g.ID,
		Name:             domain.CardgroupName(g.Name),
		Description:      g.Description,
		Language:         g.Language,
		Level:            g.Level,
		Category:         g.Category,
		CoverImageURL:    g.CoverImageURL,
		Source:           g.Source,
		Version:          g.Version,
		Status:           domain.MasterCardgroupStatus(g.Status),
		IsDefaultStarter: g.IsDefaultStarter,
		SortOrder:        g.SortOrder,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
	}
}

func masterCardgroupFromDomain(m *domain.MasterCardgroup) gormMasterCardgroup {
	return gormMasterCardgroup{
		ID:               m.ID,
		Name:             string(m.Name),
		Description:      m.Description,
		Language:         m.Language,
		Level:            m.Level,
		Category:         m.Category,
		CoverImageURL:    m.CoverImageURL,
		Source:           m.Source,
		Version:          m.Version,
		Status:           string(m.Status),
		IsDefaultStarter: m.IsDefaultStarter,
		SortOrder:        m.SortOrder,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}
