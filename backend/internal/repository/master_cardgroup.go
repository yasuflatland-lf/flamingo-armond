package repository

import (
	"context"
	"errors"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"backend/internal/domain"
)

// MasterCatalogOrderBy is the allowlist of columns the published master catalog
// may sort by. Tuple order is always (orderField, id) so cursors stay
// deterministic when the order field has duplicate values.
type MasterCatalogOrderBy string

const (
	MasterCatalogOrderBySortOrder MasterCatalogOrderBy = "sort_order"
	MasterCatalogOrderByCreatedAt MasterCatalogOrderBy = "created_at"
	MasterCatalogOrderByName      MasterCatalogOrderBy = "name"
)

// MasterCatalogCursor carries the cursor entity's id plus the column value
// matching the active orderBy. The usecase hydrates the relevant column before
// calling FindPublishedPage; an unset column for the active orderBy is a caller
// bug.
type MasterCatalogCursor struct {
	ID        string
	Name      *string
	CreatedAt *time.Time
	SortOrder *int
}

// MasterCatalogItem bundles a published master cardgroup with the number of
// master cards it contains. cardCount is derived via a correlated COUNT rather
// than a denormalized column so it can never drift from the master_cards table.
type MasterCatalogItem struct {
	Cardgroup *domain.MasterCardgroup
	CardCount int64
}

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
	// FindPublishedPage returns a window of PUBLISHED master cardgroups ordered
	// by (orderBy, id), each bundled with its card count, plus the search-aware
	// total of all matching PUBLISHED rows. The published filter is enforced in
	// SQL and is never caller-overridable. Forward paging uses (after, first);
	// backward paging uses (before, last) and the slice is reversed in memory so
	// the caller observes the same display order regardless of direction. An
	// optional case-insensitive substring search filters by name (ILIKE
	// metacharacters in the search are escaped so they match literally). The
	// returned total applies the same status + search filter as the page query
	// and is computed before the zero-page short-circuit, so a totalCount-only
	// request still observes the real count.
	FindPublishedPage(
		ctx context.Context,
		after, before *MasterCatalogCursor,
		first, last int,
		orderBy MasterCatalogOrderBy,
		dir SortOrder,
		search *string,
	) ([]*MasterCatalogItem, int64, error)
	// CountCards returns the number of master cards belonging to the given
	// master cardgroup. Used by the admin UI to display a card count per deck.
	CountCards(ctx context.Context, masterCardgroupID string) (int64, error)
	// FindPublishedByID returns the PUBLISHED master cardgroup with the given
	// id, or ErrNotFound. Draft rows return ErrNotFound — they are not part of
	// the public catalog. Used by the usecase to hydrate a pagination cursor.
	FindPublishedByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	// FindAdminPage returns a window of master cardgroups of ANY status (draft
	// or published), each bundled with its card count, plus the search-aware
	// total of all matching rows regardless of status. Unlike FindPublishedPage
	// it does not filter by status, so admin users see draft decks. All other
	// pagination, ordering, search, and totalCount semantics are identical to
	// FindPublishedPage.
	FindAdminPage(
		ctx context.Context,
		after, before *MasterCatalogCursor,
		first, last int,
		orderBy MasterCatalogOrderBy,
		dir SortOrder,
		search *string,
	) ([]*MasterCatalogItem, int64, error)
	// Publish atomically sets the master cardgroup status to published and
	// increments its version by 1. Returns ErrNotFound when no row matches.
	Publish(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	// Unpublish sets the master cardgroup status back to draft without changing
	// the version counter. Returns ErrNotFound when no row matches.
	Unpublish(ctx context.Context, id string) (*domain.MasterCardgroup, error)
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
	return masterCardgroupToDomain(row)
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
			out, err = masterCardgroupToDomain(row)
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return eris.Wrap(err, "repository: master cardgroup: ensure by name: lookup")
		}

		m, err := domain.NewMasterCardgroup(domain.CardgroupName(name), nil, nil, nil, nil, nil, nil, false, 0)
		if err != nil {
			return eris.Wrap(err, "repository: master cardgroup: ensure by name: construct")
		}
		row = masterCardgroupFromDomain(m)
		if err := tx.Create(&row).Error; err != nil {
			return eris.Wrap(err, "repository: master cardgroup: ensure by name: create")
		}
		out, err = masterCardgroupToDomain(row)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Create inserts a new master cardgroup row. The caller is responsible for
// supplying a fully-formed value (ID and both timestamps set); build it via
// domain.NewMasterCardgroup.
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
		m, err := masterCardgroupToDomain(rows[i])
		if err != nil {
			return nil, err
		}
		out[i] = m
	}
	return out, nil
}

// FindPublishedByID returns the published master cardgroup with the given id,
// or ErrNotFound. A draft row also returns ErrNotFound because the public
// catalog never exposes draft decks.
func (r *masterCardgroupRepo) FindPublishedByID(ctx context.Context, id string) (*domain.MasterCardgroup, error) {
	var row gormMasterCardgroup
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, string(domain.MasterStatusPublished)).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: master cardgroup: find published by id")
	}
	return masterCardgroupToDomain(row)
}

// Publish transitions the master cardgroup to the published state, bumping its
// version exactly once. The publish state machine and the version-bump rule live
// in domain.MasterCardgroup.Publish; this method only loads, applies, and
// persists. The row is loaded FOR UPDATE inside a transaction so concurrent
// publishes serialize and the idempotent aggregate method cannot double-bump or
// lose the version increment. Returns ErrNotFound when no row matches id.
func (r *masterCardgroupRepo) Publish(ctx context.Context, id string) (*domain.MasterCardgroup, error) {
	return r.applyStatusTransition(ctx, id, "publish", (*domain.MasterCardgroup).Publish)
}

// Unpublish transitions the master cardgroup back to draft without changing the
// version. Same load-FOR-UPDATE → aggregate-method → save transaction shape as
// Publish; the version-unchanged rule lives in domain.MasterCardgroup.Unpublish.
// Returns ErrNotFound when no row matches id.
func (r *masterCardgroupRepo) Unpublish(ctx context.Context, id string) (*domain.MasterCardgroup, error) {
	return r.applyStatusTransition(ctx, id, "unpublish", (*domain.MasterCardgroup).Unpublish)
}

// applyStatusTransition loads the master cardgroup FOR UPDATE inside a single
// transaction, applies the supplied aggregate state transition, and persists the
// resulting status and version. The row lock serializes concurrent transitions
// so the version invariant encoded in the aggregate holds without the previous
// two divergent Updates(map) statements. op is the caller-supplied verb embedded
// in the wrap prefix so the error chain attributes to publish vs. unpublish.
func (r *masterCardgroupRepo) applyStatusTransition(
	ctx context.Context, id, op string, transition func(*domain.MasterCardgroup) error,
) (*domain.MasterCardgroup, error) {
	var out *domain.MasterCardgroup
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row gormMasterCardgroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", id).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return eris.Wrap(err, "repository: master cardgroup: "+op+": load")
		}
		m, err := masterCardgroupToDomain(row)
		if err != nil {
			return err
		}
		if err := transition(m); err != nil {
			return eris.Wrap(err, "repository: master cardgroup: "+op+": apply")
		}
		if err := tx.Model(&gormMasterCardgroup{}).Where("id = ?", id).
			Updates(map[string]any{"status": string(m.Status), "version": m.Version}).Error; err != nil {
			return eris.Wrap(err, "repository: master cardgroup: "+op+": save")
		}
		out = m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func masterCardgroupToDomain(g gormMasterCardgroup) (*domain.MasterCardgroup, error) {
	status := domain.MasterCardgroupStatus(g.Status)
	if !status.IsValid() {
		return nil, eris.Errorf("repository: master cardgroup: invalid status %q", g.Status)
	}
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
		Status:           status,
		IsDefaultStarter: g.IsDefaultStarter,
		SortOrder:        g.SortOrder,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
	}, nil
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
