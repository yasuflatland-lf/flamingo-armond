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

// CardgroupCursor carries the cursor entity's id plus its `updated_at`
// value, the cardgroup connection's fixed ordering column. The usecase
// hydrates UpdatedAt before calling FindPageByOwner; an unset UpdatedAt is a
// caller bug.
type CardgroupCursor struct {
	ID        string
	UpdatedAt *time.Time
}

// gormCardgroup is the row mapping for public.cardgroups. Package-private so
// callers cannot bypass the domain conversion.
type gormCardgroup struct {
	ID        string    `gorm:"column:id;primaryKey;type:uuid"`
	OwnerID   string    `gorm:"column:owner_id"`
	Name      string    `gorm:"column:name"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;->"`
}

func (gormCardgroup) TableName() string { return "cardgroups" }

// CardgroupUpdate carries patch fields. nil means "leave untouched".
type CardgroupUpdate struct {
	Name *string
}

// ErrCardgroupOwnerNotFound is returned when an insert names an owner_id with
// no matching public.users row — in practice, a still-valid JWT whose account
// has already been deleted (auth.users delete cascades to public.users). It is
// deliberately standalone rather than joined with ErrNotFound: the cardgroup
// itself is not missing, the caller is, and callers that branch on ErrNotFound
// would otherwise misread this as "cardgroup not found".
//
// Plain errors.New (not eris) so errors.Is walks identity directly.
var ErrCardgroupOwnerNotFound = errors.New("repository: cardgroup: owner not found")

// classifyCardgroupOwnerFKError maps a Postgres FK violation (code 23503) on
// cardgroups.owner_id to ErrCardgroupOwnerNotFound. Returns nil for any other
// error so callers can use it as a pre-filter before falling through to
// eris.Wrap. Extracted as a free function so the classification can be
// unit-tested with a fabricated *pgconn.PgError without a live DB race
// (mirrors role.go's classifyFKError).
func classifyCardgroupOwnerFKError(err error) error {
	if pgConstraintViolation(err, "23503", "owner_id") {
		return ErrCardgroupOwnerNotFound
	}
	return nil
}

// CardgroupRepository provides persistence operations for the Cardgroup
// aggregate.
type CardgroupRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
	FindByName(ctx context.Context, ownerID, name string) (*domain.Cardgroup, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Cardgroup, error)
	// FindPageByOwner returns a window of cardgroups owned by ownerID ordered
	// by (updated_at DESC, id DESC) together with the total number of rows matching the same
	// owner + search filter. Paging is forward-only: at most `first` rows
	// strictly after the `after` cursor. An optional case-insensitive substring
	// search filters by name (ILIKE metacharacters in the search are escaped so
	// they match literally); the returned total honours that same filter, so it
	// never lies under an active search. The COUNT runs before the zero-page
	// short-circuit so a caller requesting only the total still sees a real
	// value.
	FindPageByOwner(
		ctx context.Context,
		ownerID string,
		after *CardgroupCursor,
		first int,
		search *string,
	) ([]*domain.Cardgroup, int64, error)
	// CountByOwner returns the total number of cardgroups owned by ownerID
	// matching the optional search predicate. Used by the filter-less callers
	// that pass a nil search (the stats dashboard); the paginated connection
	// reads its filtered total from FindPageByOwner instead.
	CountByOwner(ctx context.Context, ownerID string, search *string) (int64, error)
	// CountByOwnerTx counts every cardgroup owned by ownerID on tx, so a caller
	// holding AcquireUserCardgroupLockTx reads the count on the locked connection.
	CountByOwnerTx(ctx context.Context, tx *gorm.DB, ownerID string) (int64, error)
	Create(ctx context.Context, cg *domain.Cardgroup) error
	// CreateTx inserts a new cardgroup row using the supplied transaction
	// handle so the insert participates in the caller's transaction. The caller
	// is responsible for pre-filling cg.ID (uuid v7) and CreatedAt. UpdatedAt is
	// assigned by the database and copied back into cg.
	CreateTx(ctx context.Context, tx *gorm.DB, cg *domain.Cardgroup) error
	// AcquireUserCardgroupLockTx takes a per-owner transaction-scoped advisory
	// lock. Every write that adds cardgroups to an owner under the general-user
	// quota (create, master import, default-starter seed) takes it first, so the
	// count-then-insert sequences for one owner serialize.
	AcquireUserCardgroupLockTx(ctx context.Context, tx *gorm.DB, userID string) error
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
		// Do not apply SQLSTATE 22P02 where another client-controlled bind could fail; id is the only one here.
		if errors.Is(err, gorm.ErrRecordNotFound) || pgInvalidTextRepresentation(err) {
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
// ownerID. Order is (updated_at DESC, id DESC) so cursors stay deterministic
// even when two rows share an updated_at. The search argument, when
// non-empty after trimming, filters by `name ILIKE %escaped%` with LIKE
// metacharacters escaped so user-supplied `%` and `_` match literally.
// totalCount is a COUNT(*) over the SAME filtered base query, so it honours
// the active search rather than reporting the unfiltered owner total.
func (r *cardgroupRepo) FindPageByOwner(
	ctx context.Context,
	ownerID string,
	after *CardgroupCursor,
	first int,
	search *string,
) ([]*domain.Cardgroup, int64, error) {
	first = ClampPageSize(first)

	// Base query scoped to the owner (and search filter, if active). Both the
	// COUNT and the page window derive from it so totalCount applies the same
	// predicate as the page.
	base := r.db.WithContext(ctx).Model(&gormCardgroup{}).Where("owner_id = ?", ownerID)
	if pattern, ok := searchLikePattern(search); ok {
		base = base.Where("name ILIKE ?", pattern)
	}

	// totalCount comes from a COUNT(*) over the same filtered base query.
	// Computed before the no-rows short-circuit so a caller passing first=0
	// still observes the real count.
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: cardgroup: count by owner")
	}

	if first == 0 {
		return []*domain.Cardgroup{}, total, nil
	}

	q := base
	if after != nil {
		clauseSQL, args, err := cardgroupCursorWhere(after)
		if err != nil {
			return nil, 0, eris.Wrap(err, "repository: cardgroup: build cursor where")
		}
		q = q.Where(clauseSQL, args...)
	}
	q = q.Order(cardgroupOrderClause()).Limit(first)

	var rows []gormCardgroup
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: cardgroup: find page by owner")
	}

	out := make([]*domain.Cardgroup, len(rows))
	for i := range rows {
		out[i] = cardgroupToDomain(rows[i])
	}
	return out, total, nil
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

// CountByOwnerTx counts every cardgroup owned by ownerID on the supplied tx.
func (r *cardgroupRepo) CountByOwnerTx(ctx context.Context, tx *gorm.DB, ownerID string) (int64, error) {
	var total int64
	if err := tx.WithContext(ctx).Model(&gormCardgroup{}).Where("owner_id = ?", ownerID).Count(&total).Error; err != nil {
		return 0, eris.Wrap(err, "repository: cardgroup: count by owner tx")
	}
	return total, nil
}

// cardgroupCursorSpec describes the cardgroup aggregate's cursor geometry: the
// fixed (updated_at DESC, id DESC) ordering. The columns are UNALIASED because
// FindPageByOwner queries the cardgroups table without an alias.
func cardgroupCursorSpec(c *CardgroupCursor) cursorSpec {
	return cursorSpec{
		alias:      "",
		orderCol:   "updated_at",
		isIDOrder:  false,
		fieldValue: func() (any, error) { return cardgroupCursorFieldValue(c) },
	}
}

// cardgroupOrderClause renders the SQL ORDER BY tail; the secondary `id` keeps
// the ordering total.
func cardgroupOrderClause() string {
	return buildOrderClause(cardgroupCursorSpec(nil), SortDesc)
}

// cardgroupCursorWhere builds the tuple-comparison WHERE for the supplied
// cursor. Returns an error when the cursor lacks UpdatedAt — that is a caller
// bug, not user-supplied input.
func cardgroupCursorWhere(c *CardgroupCursor) (string, []any, error) {
	return buildCursorWhere(cardgroupCursorSpec(c), SortDesc, c.ID)
}

// cardgroupCursorFieldValue returns the cursor's updated_at value. The usecase
// layer hydrates it before calling FindPageByOwner, so a nil is a caller bug.
func cardgroupCursorFieldValue(c *CardgroupCursor) (any, error) {
	if c.UpdatedAt != nil {
		return *c.UpdatedAt, nil
	}
	return nil, eris.New("repository: cardgroup: cursor missing updated_at column")
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
// cg.ID (uuid v7) and CreatedAt. UpdatedAt is assigned by the database and
// copied back into cg. An owner_id that no longer resolves to a
// public.users row returns ErrCardgroupOwnerNotFound rather than an opaque
// internal error.
func (r *cardgroupRepo) Create(ctx context.Context, cg *domain.Cardgroup) error {
	row := cardgroupToRow(cg)
	if err := r.db.WithContext(ctx).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "updated_at"}}}).
		Create(row).Error; err != nil {
		if fkErr := classifyCardgroupOwnerFKError(err); fkErr != nil {
			return fkErr
		}
		if classified := classifyTextLengthViolation(err); classified != nil {
			return classified
		}
		return eris.Wrap(err, "repository: cardgroup: create")
	}
	cg.UpdatedAt = row.UpdatedAt
	return nil
}

// CreateTx inserts a new cardgroup row using the supplied transaction handle so
// the insert participates in the caller's transaction. The caller is
// responsible for pre-filling cg.ID (uuid v7) and CreatedAt. UpdatedAt is
// assigned by the database and copied back into cg. An owner_id that no longer
// resolves to a public.users row returns ErrCardgroupOwnerNotFound
// rather than an opaque internal error — the master-deck copy paths write the
// caller-owned cardgroup through this method, so a deleted account importing or
// seeding a starter deck must reach the same classification as Create.
func (r *cardgroupRepo) CreateTx(ctx context.Context, tx *gorm.DB, cg *domain.Cardgroup) error {
	row := cardgroupToRow(cg)
	if err := tx.WithContext(ctx).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "updated_at"}}}).
		Create(row).Error; err != nil {
		if fkErr := classifyCardgroupOwnerFKError(err); fkErr != nil {
			return fkErr
		}
		if classified := classifyTextLengthViolation(err); classified != nil {
			return classified
		}
		return eris.Wrap(err, "repository: cardgroup: create tx")
	}
	cg.UpdatedAt = row.UpdatedAt
	return nil
}

// AcquireUserCardgroupLockTx takes a per-user advisory lock (released at tx end)
// so every quota-bound cardgroup write for the same owner (create, master
// import, default-starter seed) serializes. hashtext returns int4, so
// pg_advisory_xact_lock(0, hashtext(userID)) keys the lock on the user within a
// fixed namespace where unrelated callers do not contend.
func (r *cardgroupRepo) AcquireUserCardgroupLockTx(ctx context.Context, tx *gorm.DB, userID string) error {
	if err := tx.WithContext(ctx).Exec("SELECT pg_advisory_xact_lock(0, hashtext(?))", userID).Error; err != nil {
		return eris.Wrap(err, "repository: cardgroup: acquire user cardgroup lock")
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
		if classified := classifyTextLengthViolation(res.Error); classified != nil {
			return nil, classified
		}
		return nil, eris.Wrap(res.Error, "repository: cardgroup: update")
	}
	// Re-fetch so callers see the trigger-refreshed updated_at.
	return refetchAfterUpdate(res.RowsAffected, ErrNotFound,
		func() (*domain.Cardgroup, error) { return r.FindByID(ctx, id) }, "")
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
