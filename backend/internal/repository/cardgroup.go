package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// CardgroupOrderBy is the allowlist of columns that paginated cardgroup
// queries may sort by. Tuple order is always (orderField, id) so cursors
// stay deterministic when the order field has duplicate values.
type CardgroupOrderBy string

const (
	CardgroupOrderByID        CardgroupOrderBy = "id"
	CardgroupOrderByCreatedAt CardgroupOrderBy = "created_at"
	CardgroupOrderByUpdatedAt CardgroupOrderBy = "updated_at"
	CardgroupOrderByName      CardgroupOrderBy = "name"
)

// CardgroupCursor carries the cursor entity's id plus the column value
// matching the active orderBy. The usecase hydrates the relevant column
// before calling FindPageByOwner; an unset column for the active orderBy
// is a caller bug.
type CardgroupCursor struct {
	ID        string
	Name      *string
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

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
	FindByName(ctx context.Context, ownerID, name string) (*domain.Cardgroup, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Cardgroup, error)
	// FindPageByOwner returns a window of cardgroups owned by ownerID
	// ordered by (orderBy, id). Forward paging uses (after, first); backward
	// paging uses (before, last) and the slice is reversed in memory so the
	// caller observes the same display order regardless of direction. An
	// optional case-insensitive substring search filters by name (ILIKE
	// metacharacters in the search are escaped so they match literally).
	FindPageByOwner(
		ctx context.Context,
		ownerID string,
		after, before *CardgroupCursor,
		first, last int,
		orderBy CardgroupOrderBy,
		dir SortOrder,
		search *string,
	) ([]*domain.Cardgroup, error)
	// CountByOwner returns the total number of cardgroups owned by ownerID
	// matching the optional search predicate. Returned independently of
	// FindPageByOwner so the totalCount survives a zero-page request.
	CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error)
	Create(ctx context.Context, cg *domain.Cardgroup) error
	// CreateTx inserts a new cardgroup row using the supplied transaction
	// handle so the insert participates in the caller's transaction. The caller
	// is responsible for pre-filling cg.ID (uuid v7) and both timestamps.
	CreateTx(ctx context.Context, tx *gorm.DB, cg *domain.Cardgroup) error
	EnsureByName(ctx context.Context, ownerID, name string) (*domain.Cardgroup, error)
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
		return nil, eris.Wrap(err, "repository: cardgroup: find by id")
	}
	return cardgroupToDomain(row), nil
}

// FindByName returns the cardgroup identified by the (owner_id, name) pair, or
// ErrNotFound when no such row exists.
func (r *cardgroupRepo) FindByName(ctx context.Context, ownerID, name string) (*domain.Cardgroup, error) {
	var row gormCardgroup
	err := r.db.WithContext(ctx).
		Where("owner_id = ? AND name = ?", ownerID, name).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: cardgroup: find by name")
	}
	return cardgroupToDomain(row), nil
}

// FindPageByOwner implements the cursor-paginated cardgroup list scoped to
// ownerID. Order is (orderBy, id) so cursors stay deterministic even when
// the primary sort column has duplicates. Backward paging inverts the SQL
// direction, applies LIMIT, and reverses the slice in memory so the caller
// sees the same display order as forward paging. The search argument, when
// non-empty after trimming, filters by `name ILIKE %escaped%` with LIKE
// metacharacters escaped so user-supplied `%` and `_` match literally.
func (r *cardgroupRepo) FindPageByOwner(
	ctx context.Context,
	ownerID string,
	after, before *CardgroupCursor,
	first, last int,
	orderBy CardgroupOrderBy,
	dir SortOrder,
	search *string,
) ([]*domain.Cardgroup, error) {
	first = ClampPageSize(first)
	last = ClampPageSize(last)

	if first == 0 && last == 0 {
		return []*domain.Cardgroup{}, nil
	}

	// Backward paging executes the query with the inverted direction and
	// reverses the slice afterwards.
	effectiveDir, limit, cursor, reverse := paginateSetup(dir, first, last, after, before)

	q := r.db.WithContext(ctx).Model(&gormCardgroup{}).Where("owner_id = ?", ownerID)
	if pattern, ok := searchLikePattern(search); ok {
		q = q.Where("name ILIKE ?", pattern)
	}
	if cursor != nil {
		clauseSQL, args, err := cardgroupCursorWhere(orderBy, effectiveDir, cursor)
		if err != nil {
			return nil, eris.Wrap(err, "repository: cardgroup: build cursor where")
		}
		q = q.Where(clauseSQL, args...)
	}
	q = q.Order(cardgroupOrderClause(orderBy, effectiveDir)).Limit(limit)

	var rows []gormCardgroup
	if err := q.Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: cardgroup: find page by owner")
	}

	if reverse {
		ReverseSlice(rows)
	}

	out := make([]*domain.Cardgroup, len(rows))
	for i := range rows {
		out[i] = cardgroupToDomain(rows[i])
	}
	return out, nil
}

// CountByOwner returns the total number of cardgroups owned by ownerID
// matching the optional search predicate. The COUNT scopes by owner_id so
// a tenant cannot observe other tenants' aggregate sizes.
func (r *cardgroupRepo) CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&gormCardgroup{}).Where("owner_id = ?", ownerID)
	if pattern, ok := searchLikePattern(search); ok {
		q = q.Where("name ILIKE ?", pattern)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, eris.Wrap(err, "repository: cardgroup: count by owner")
	}
	return total, nil
}

// cardgroupOrderClause renders the SQL ORDER BY tail. When orderBy is `id`
// only one column appears; otherwise the secondary `id` keeps the ordering
// total.
func cardgroupOrderClause(orderBy CardgroupOrderBy, dir SortOrder) string {
	if orderBy == CardgroupOrderByID {
		return "id " + string(dir)
	}
	return string(orderBy) + " " + string(dir) + ", id " + string(dir)
}

// cardgroupCursorWhere builds the tuple-comparison WHERE for the supplied
// cursor and direction. ASC yields `>`, DESC yields `<`. Returns an error
// when the cursor lacks the column required by the active orderBy — that
// is a caller bug, not user-supplied input.
func cardgroupCursorWhere(orderBy CardgroupOrderBy, dir SortOrder, c *CardgroupCursor) (string, []any, error) {
	op := ">"
	if dir == SortDesc {
		op = "<"
	}
	if orderBy == CardgroupOrderByID {
		return "id " + op + " ?", []any{c.ID}, nil
	}
	field := string(orderBy)
	val, err := cardgroupCursorFieldValue(orderBy, c)
	if err != nil {
		return "", nil, err
	}
	clause, args := cursorTupleWhere("", field, op, val, c.ID)
	return clause, args, nil
}

// cardgroupCursorFieldValue returns the cursor value for the active
// orderBy field. The usecase layer hydrates the relevant column before
// calling FindPageByOwner, so a missing column is a caller bug.
func cardgroupCursorFieldValue(orderBy CardgroupOrderBy, c *CardgroupCursor) (any, error) {
	switch orderBy {
	case CardgroupOrderByName:
		if c.Name != nil {
			return *c.Name, nil
		}
	case CardgroupOrderByCreatedAt:
		if c.CreatedAt != nil {
			return *c.CreatedAt, nil
		}
	case CardgroupOrderByUpdatedAt:
		if c.UpdatedAt != nil {
			return *c.UpdatedAt, nil
		}
	}
	return nil, eris.Errorf("cardgroup cursor missing %s column", orderBy)
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
		return nil, eris.Wrap(err, "repository: cardgroup: find by ids")
	}
	out := make(map[string]*domain.Cardgroup, len(rows))
	for i := range rows {
		cg := cardgroupToDomain(rows[i])
		out[string(cg.ID)] = cg
	}
	return out, nil
}

// Create inserts a new cardgroup row. The caller is responsible for pre-filling
// cg.ID (uuid v7) and both timestamps.
func (r *cardgroupRepo) Create(ctx context.Context, cg *domain.Cardgroup) error {
	row := cardgroupToRow(cg)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return eris.Wrap(err, "repository: cardgroup: create")
	}
	return nil
}

// CreateTx inserts a new cardgroup row using the supplied transaction handle so
// the insert participates in the caller's transaction. The caller is
// responsible for pre-filling cg.ID (uuid v7) and both timestamps.
func (r *cardgroupRepo) CreateTx(ctx context.Context, tx *gorm.DB, cg *domain.Cardgroup) error {
	if err := tx.WithContext(ctx).Create(cardgroupToRow(cg)).Error; err != nil {
		return eris.Wrap(err, "repository: cardgroup: create tx")
	}
	return nil
}

// EnsureByName returns the existing (owner_id, name) cardgroup or creates it
// when absent. The cardgroups table has no UNIQUE(owner_id, name) constraint,
// so two concurrent callers could otherwise insert duplicate (owner_id, name)
// rows. This method takes a transaction-scoped advisory lock keyed on
// (owner_id, name) to serialize lookup-then-insert without adding a DB-level
// constraint that would change the public duplicate-name semantics.
func (r *cardgroupRepo) EnsureByName(ctx context.Context, ownerID, name string) (*domain.Cardgroup, error) {
	var out *domain.Cardgroup
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?), hashtext(?))", ownerID, name).Error; err != nil {
			return eris.Wrap(err, "repository: cardgroup: ensure by name: advisory lock")
		}

		var row gormCardgroup
		err := tx.Where("owner_id = ? AND name = ?", ownerID, name).Take(&row).Error
		if err == nil {
			out = cardgroupToDomain(row)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return eris.Wrap(err, "repository: cardgroup: ensure by name: lookup")
		}

		id, err := uuid.NewV7()
		if err != nil {
			return eris.Wrap(err, "repository: cardgroup: ensure by name: uuid")
		}
		now := time.Now().UTC()
		row = gormCardgroup{
			ID:        id.String(),
			OwnerID:   ownerID,
			Name:      name,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := tx.Create(&row).Error; err != nil {
			return eris.Wrap(err, "repository: cardgroup: ensure by name: create")
		}
		out = cardgroupToDomain(row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
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
		return nil, eris.Wrap(res.Error, "repository: cardgroup: update")
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
		return eris.Wrap(res.Error, "repository: cardgroup: delete")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func cardgroupToRow(cg *domain.Cardgroup) *gormCardgroup {
	return &gormCardgroup{
		ID:        string(cg.ID),
		OwnerID:   string(cg.OwnerID),
		Name:      string(cg.Name),
		CreatedAt: cg.CreatedAt,
		UpdatedAt: cg.UpdatedAt,
	}
}

func cardgroupToDomain(g gormCardgroup) *domain.Cardgroup {
	return &domain.Cardgroup{
		ID:        domain.CardgroupID(g.ID),
		OwnerID:   domain.UserID(g.OwnerID),
		Name:      domain.CardgroupName(g.Name),
		CreatedAt: g.CreatedAt,
		UpdatedAt: g.UpdatedAt,
	}
}
