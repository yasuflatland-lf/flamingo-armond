package repository

import (
	"context"
	"errors"
	"strings"
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
	// by (orderBy, id), each bundled with its card count. The published filter
	// is enforced in SQL and is never caller-overridable. Forward paging uses
	// (after, first); backward paging uses (before, last) and the slice is
	// reversed in memory so the caller observes the same display order
	// regardless of direction. An optional case-insensitive substring search
	// filters by name (ILIKE metacharacters in the search are escaped so they
	// match literally).
	FindPublishedPage(
		ctx context.Context,
		after, before *MasterCatalogCursor,
		first, last int,
		orderBy MasterCatalogOrderBy,
		dir SortOrder,
		search *string,
	) ([]*MasterCatalogItem, error)
	// CountPublished returns the total number of PUBLISHED master cardgroups
	// matching the optional search predicate. Returned independently of
	// FindPublishedPage so the totalCount survives a zero-page request.
	CountPublished(ctx context.Context, search *string) (int64, error)
	// CountAdmin returns the total number of master cardgroups of ANY status
	// (draft or published) matching the optional search predicate. Used by the
	// admin list to display totalCount regardless of publication status.
	CountAdmin(ctx context.Context, search *string) (int64, error)
	// CountCards returns the number of master cards belonging to the given
	// master cardgroup. Used by the admin UI to display a card count per deck.
	CountCards(ctx context.Context, masterCardgroupID string) (int64, error)
	// FindPublishedByID returns the PUBLISHED master cardgroup with the given
	// id, or ErrNotFound. Draft rows return ErrNotFound — they are not part of
	// the public catalog. Used by the usecase to hydrate a pagination cursor.
	FindPublishedByID(ctx context.Context, id string) (*domain.MasterCardgroup, error)
	// FindAdminPage returns a window of master cardgroups of ANY status (draft
	// or published), each bundled with its card count. Unlike FindPublishedPage
	// it does not filter by status, so admin users see draft decks. All other
	// pagination, ordering, and search semantics are identical to
	// FindPublishedPage.
	FindAdminPage(
		ctx context.Context,
		after, before *MasterCatalogCursor,
		first, last int,
		orderBy MasterCatalogOrderBy,
		dir SortOrder,
		search *string,
	) ([]*MasterCatalogItem, error)
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

// gormMasterCatalogRow is the flat scan target for FindPublishedPage. It holds
// every master_cardgroups column as a flat field plus the derived card_count
// produced by the correlated COUNT. Embedding gormMasterCardgroup is
// intentionally avoided: gormMasterCardgroup carries a TableName() method that
// confuses GORM's embedded-struct schema parser when the outer scan target is a
// different type, silently leaving every embedded column at its Go zero value.
// See docs/backend/library-gotchas/gorm-embedded-tablename-scan-confusion.md.
type gormMasterCatalogRow struct {
	ID               string    `gorm:"column:id"`
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
	CardCount        int64     `gorm:"column:card_count"`
}

// toGorm rebuilds the gormMasterCardgroup view of the row so the shared
// masterCardgroupToDomain conversion remains the single domain boundary.
func (r gormMasterCatalogRow) toGorm() gormMasterCardgroup {
	return gormMasterCardgroup{
		ID:               r.ID,
		Name:             r.Name,
		Description:      r.Description,
		Language:         r.Language,
		Level:            r.Level,
		Category:         r.Category,
		CoverImageURL:    r.CoverImageURL,
		Source:           r.Source,
		Version:          r.Version,
		Status:           r.Status,
		IsDefaultStarter: r.IsDefaultStarter,
		SortOrder:        r.SortOrder,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
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
	return masterCardgroupToDomain(row), nil
}

// CountPublished returns the total number of published master cardgroups
// matching the optional search predicate. The published filter is enforced in
// SQL and is never caller-overridable.
func (r *masterCardgroupRepo) CountPublished(ctx context.Context, search *string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&gormMasterCardgroup{}).
		Where("status = ?", string(domain.MasterStatusPublished))
	if pattern, ok := masterCatalogSearchPattern(search); ok {
		q = q.Where("name ILIKE ?", pattern)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, eris.Wrap(err, "repository: master cardgroup: count published")
	}
	return total, nil
}

// FindPublishedPage implements the cursor-paginated published catalog list.
// Order is (orderBy, id) so cursors stay deterministic even when the primary
// sort column has duplicates. Backward paging inverts the SQL direction,
// applies LIMIT, and reverses the slice in memory so the caller sees the same
// display order as forward paging. cardCount is computed via a correlated
// COUNT over master_cards so it can never drift from a denormalized column.
func (r *masterCardgroupRepo) FindPublishedPage(
	ctx context.Context,
	after, before *MasterCatalogCursor,
	first, last int,
	orderBy MasterCatalogOrderBy,
	dir SortOrder,
	search *string,
) ([]*MasterCatalogItem, error) {
	first = ClampPageSize(first)
	last = ClampPageSize(last)

	if first == 0 && last == 0 {
		return []*MasterCatalogItem{}, nil
	}

	// Backward paging executes the query with the inverted direction and
	// reverses the slice afterwards.
	effectiveDir := dir
	limit := first
	cursor := after
	reverse := false
	if last > 0 {
		effectiveDir = InvertDir(dir)
		limit = last
		cursor = before
		reverse = true
	}

	q := r.db.WithContext(ctx).
		Table("master_cardgroups AS mcg").
		Select("mcg.*, (SELECT COUNT(*) FROM master_cards mc WHERE mc.master_cardgroup_id = mcg.id) AS card_count").
		Where("mcg.status = ?", string(domain.MasterStatusPublished))
	if pattern, ok := masterCatalogSearchPattern(search); ok {
		q = q.Where("mcg.name ILIKE ?", pattern)
	}
	if cursor != nil {
		clauseSQL, args, err := masterCatalogCursorWhere(orderBy, effectiveDir, cursor)
		if err != nil {
			return nil, eris.Wrap(err, "repository: master cardgroup: build catalog cursor where")
		}
		q = q.Where(clauseSQL, args...)
	}
	q = q.Order(masterCatalogOrderClause(orderBy, effectiveDir)).Limit(limit)

	var rows []gormMasterCatalogRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: master cardgroup: find published page")
	}

	if reverse {
		ReverseSlice(rows)
	}

	out := make([]*MasterCatalogItem, len(rows))
	for i := range rows {
		out[i] = &MasterCatalogItem{
			Cardgroup: masterCardgroupToDomain(rows[i].toGorm()),
			CardCount: rows[i].CardCount,
		}
	}
	return out, nil
}

// masterCatalogSearchPattern wraps a non-empty trimmed search term in `%...%`
// after escaping LIKE metacharacters. Returns (pattern, false) when the search
// is nil or trims empty so callers can skip the predicate.
func masterCatalogSearchPattern(search *string) (string, bool) {
	if search == nil {
		return "", false
	}
	trimmed := strings.TrimSpace(*search)
	if trimmed == "" {
		return "", false
	}
	return "%" + escapeLikePattern(trimmed) + "%", true
}

// masterCatalogOrderClause renders the SQL ORDER BY tail. When orderBy is `id`
// only one column appears; otherwise the secondary `id` keeps the ordering
// total. The columns are prefixed with the `mcg` alias used by
// FindPublishedPage.
func masterCatalogOrderClause(orderBy MasterCatalogOrderBy, dir SortOrder) string {
	return "mcg." + string(orderBy) + " " + string(dir) + ", mcg.id " + string(dir)
}

// masterCatalogCursorWhere builds the tuple-comparison WHERE for the supplied
// cursor and direction. ASC yields `>`, DESC yields `<`. Returns an error when
// the cursor lacks the column required by the active orderBy — that is a caller
// bug, not user-supplied input. Columns are prefixed with the `mcg` alias used
// by FindPublishedPage.
func masterCatalogCursorWhere(orderBy MasterCatalogOrderBy, dir SortOrder, c *MasterCatalogCursor) (string, []any, error) {
	op := ">"
	if dir == SortDesc {
		op = "<"
	}
	field := "mcg." + string(orderBy)
	val, err := masterCatalogCursorFieldValue(orderBy, c)
	if err != nil {
		return "", nil, err
	}
	// Tuple compare: (field, id) op (val, c.ID). Expanded form is portable
	// across SQL dialects (Postgres-only `(a, b) > (?, ?)` avoided).
	return "(" + field + " " + op + " ? OR (" + field + " = ? AND mcg.id " + op + " ?))",
		[]any{val, val, c.ID}, nil
}

// masterCatalogCursorFieldValue returns the cursor value for the active orderBy
// field. The usecase layer hydrates the relevant column before calling
// FindPublishedPage, so a missing column is a caller bug.
func masterCatalogCursorFieldValue(orderBy MasterCatalogOrderBy, c *MasterCatalogCursor) (any, error) {
	switch orderBy {
	case MasterCatalogOrderBySortOrder:
		if c.SortOrder != nil {
			return *c.SortOrder, nil
		}
	case MasterCatalogOrderByCreatedAt:
		if c.CreatedAt != nil {
			return *c.CreatedAt, nil
		}
	case MasterCatalogOrderByName:
		if c.Name != nil {
			return *c.Name, nil
		}
	}
	return nil, eris.Errorf("repository: master cardgroup: catalog cursor missing %s column", orderBy)
}

// CountAdmin returns the total number of master cardgroups of any status
// (draft or published) matching the optional search predicate.
func (r *masterCardgroupRepo) CountAdmin(ctx context.Context, search *string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&gormMasterCardgroup{})
	if pattern, ok := masterCatalogSearchPattern(search); ok {
		q = q.Where("name ILIKE ?", pattern)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, eris.Wrap(err, "repository: master cardgroup: count admin")
	}
	return total, nil
}

// CountCards returns the number of master cards belonging to the given master
// cardgroup.
func (r *masterCardgroupRepo) CountCards(ctx context.Context, masterCardgroupID string) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Table("master_cards").
		Where("master_cardgroup_id = ?", masterCardgroupID).
		Count(&total).Error; err != nil {
		return 0, eris.Wrap(err, "repository: master cardgroup: count cards")
	}
	return total, nil
}

// FindAdminPage implements the cursor-paginated admin catalog list. It is
// identical to FindPublishedPage but does NOT filter by status, so draft decks
// are included. Order is (orderBy, id) so cursors stay deterministic even when
// the primary sort column has duplicates. Backward paging inverts the SQL
// direction, applies LIMIT, and reverses the slice in memory so the caller sees
// the same display order as forward paging.
func (r *masterCardgroupRepo) FindAdminPage(
	ctx context.Context,
	after, before *MasterCatalogCursor,
	first, last int,
	orderBy MasterCatalogOrderBy,
	dir SortOrder,
	search *string,
) ([]*MasterCatalogItem, error) {
	first = ClampPageSize(first)
	last = ClampPageSize(last)

	if first == 0 && last == 0 {
		return []*MasterCatalogItem{}, nil
	}

	// Backward paging executes the query with the inverted direction and
	// reverses the slice afterwards.
	effectiveDir := dir
	limit := first
	cursor := after
	reverse := false
	if last > 0 {
		effectiveDir = InvertDir(dir)
		limit = last
		cursor = before
		reverse = true
	}

	q := r.db.WithContext(ctx).
		Table("master_cardgroups AS mcg").
		Select("mcg.*, (SELECT COUNT(*) FROM master_cards mc WHERE mc.master_cardgroup_id = mcg.id) AS card_count")
	if pattern, ok := masterCatalogSearchPattern(search); ok {
		q = q.Where("mcg.name ILIKE ?", pattern)
	}
	if cursor != nil {
		clauseSQL, args, err := masterCatalogCursorWhere(orderBy, effectiveDir, cursor)
		if err != nil {
			return nil, eris.Wrap(err, "repository: master cardgroup: build admin cursor where")
		}
		q = q.Where(clauseSQL, args...)
	}
	q = q.Order(masterCatalogOrderClause(orderBy, effectiveDir)).Limit(limit)

	var rows []gormMasterCatalogRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: master cardgroup: find admin page")
	}

	if reverse {
		ReverseSlice(rows)
	}

	out := make([]*MasterCatalogItem, len(rows))
	for i := range rows {
		out[i] = &MasterCatalogItem{
			Cardgroup: masterCardgroupToDomain(rows[i].toGorm()),
			CardCount: rows[i].CardCount,
		}
	}
	return out, nil
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
		m := masterCardgroupToDomain(row)
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
