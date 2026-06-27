package repository

import (
	"context"
	"errors"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// ErrMasterCardgroupNotFound is returned by Create / UpsertManyTx when the
// target master_cardgroup_id does not exist — a Postgres FK violation (23503)
// on master_cards_master_cardgroup_id_fkey. Joined with ErrNotFound so callers
// matching the general sentinel keep working while new callers branch on the
// specific cause to surface BAD_USER_INPUT on the masterCardgroupId field.
//
// Plain errors.New (not eris) so errors.Is walks identity directly.
var ErrMasterCardgroupNotFound = errors.Join(
	errors.New("repository: master card: master cardgroup not found"),
	ErrNotFound,
)

// classifyMasterCardFKError maps a Postgres FK violation (code 23503) on the
// master_cards.master_cardgroup_id foreign key to ErrMasterCardgroupNotFound.
// Returns nil for any other error so callers can use it as a pre-filter before
// falling through to eris.Wrap. Extracted as a free function so the
// classification can be unit-tested with a fabricated *pgconn.PgError without a
// live DB race (mirrors role.go's classifyFKError).
func classifyMasterCardFKError(err error) error {
	if pgConstraintViolation(err, "23503", "master_cardgroup_id") {
		return ErrMasterCardgroupNotFound
	}
	return nil
}

// gormMasterCard is the row mapping for public.master_cards. Package-private so
// callers cannot bypass the domain conversion. It is intentionally NOT embedded
// in any outer scan target: gormMasterCard carries a TableName() method that
// confuses GORM's embedded-struct schema parser when the outer scan target is a
// different type (see `.claude/rules/go-library-gotchas.md`).
type gormMasterCard struct {
	ID                string    `gorm:"column:id;primaryKey;type:uuid"`
	MasterCardgroupID string    `gorm:"column:master_cardgroup_id"`
	Front             string    `gorm:"column:front"`
	Back              string    `gorm:"column:back"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
	Position          int       `gorm:"column:position"`
}

func (gormMasterCard) TableName() string { return "master_cards" }

// MasterCardOrderBy is the allowlist of columns paginated master-card queries may
// sort by, mirroring CardOrderBy. The string values are the snake_case column
// names; the GraphQL MasterCardOrderBy enum (ID / POSITION / CREATED_AT /
// UPDATED_AT) maps onto these in the usecase layer. Lexicographic tuple order is
// always (orderField, id) so cursors stay deterministic even when the order field
// has duplicate values.
type MasterCardOrderBy string

const (
	MasterCardOrderByID        MasterCardOrderBy = "id"
	MasterCardOrderByPosition  MasterCardOrderBy = "position"
	MasterCardOrderByCreatedAt MasterCardOrderBy = "created_at"
	MasterCardOrderByUpdatedAt MasterCardOrderBy = "updated_at"
)

// MasterCardCursor is an opaque cursor for paginated master-card queries,
// mirroring CardCursor. Only the field relevant to the active OrderBy needs to be
// populated; ID is always populated and acts as the secondary key in the tuple
// comparison. Position is an int (the POSITION ordering column), so it has its own
// field separate from the time-typed columns.
type MasterCardCursor struct {
	ID        string
	Position  *int
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

// MasterCardUpdate is the field-patch payload for masterCardRepo.Update. A nil
// pointer means "leave the column unchanged"; a non-nil pointer overwrites the
// column. Mirrors repository.CardUpdate.
type MasterCardUpdate struct {
	Front *string
	Back  *string
}

// MasterCardRepository provides persistence operations for the MasterCard
// aggregate. The bulk Tx methods share the table-parameterized helpers in
// card.go (upsertManyTx / listFrontsByCardgroupTx / deleteByCardgroupAndFrontsTx)
// with "master_cards" and "master_cardgroup_id".
type MasterCardRepository interface {
	ListByMasterCardgroup(ctx context.Context, masterCardgroupID string) ([]*domain.MasterCard, error)
	// FindByID returns the master card with the given primary key, or ErrNotFound
	// when no such row exists. The usecase uses it to hydrate a pagination cursor's
	// ordering column (position / created_at / updated_at) for a single decoded
	// cursor id; the cross-group guard is enforced by the usecase, not here.
	FindByID(ctx context.Context, id string) (*domain.MasterCard, error)
	// FindByMasterCardgroupAndFront returns the master card identified by the
	// (master_cardgroup_id, front) unique key, or ErrNotFound when no such row
	// exists. front matches case-insensitively (the column is citext). The admin
	// create path uses it to hydrate the existing card after a duplicate-front
	// 23505 collision.
	FindByMasterCardgroupAndFront(ctx context.Context, masterCardgroupID, front string) (*domain.MasterCard, error)
	// Update applies a field patch and returns the updated row, or ErrNotFound
	// when no row matches the id. An all-nil patch is a no-op that returns the
	// current row. Mirrors cardRepo.Update.
	Update(ctx context.Context, id string, patch MasterCardUpdate) (*domain.MasterCard, error)
	// DeleteMany hard-deletes the master cards whose ids are in the list and
	// returns the number of rows actually deleted. Master decks are admin-owned
	// and global, so there is no owner scope (unlike cardRepo.DeleteByIDsTx).
	//
	// Empty ids short-circuits to (0, nil) without touching the DB. With an empty
	// slice GORM v2 omits the `WHERE id IN (?)` clause altogether, which would
	// convert this Delete into an unbounded mass delete — see
	// `.claude/rules/go-library-gotchas.md` § GORM empty IN.
	DeleteMany(ctx context.Context, ids []string) (int64, error)
	// FindPageByMasterCardgroup returns a window of master cards for a master
	// cardgroup ordered by (orderField, id). Forward paging uses after + first;
	// backward paging uses before + last. The returned totalCount is search-aware:
	// it reflects every row in the group AND the search filter when one is active,
	// not just the page. The usecase consumes this totalCount directly. The return
	// shape mirrors cardRepo.FindPageByCardgroupForUser so the usecase page helpers
	// (assemblePage / TrimAndDetect) consume it identically — master cards carry no
	// per-viewer / FSRS state, so there is no userID parameter.
	FindPageByMasterCardgroup(
		ctx context.Context,
		masterCardgroupID string,
		after, before *MasterCardCursor,
		first, last int,
		orderBy MasterCardOrderBy,
		dir SortOrder,
		search *string,
	) (cards []*domain.MasterCard, totalCount int64, err error)
	Create(ctx context.Context, c *domain.MasterCard) error
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.MasterCard) (UpsertManyTxResult, error)
	ListFrontsByMasterCardgroupTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string) ([]string, error)
	DeleteByMasterCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string, fronts []string) (int64, error)
	Delete(ctx context.Context, id string) error
}

type masterCardRepo struct{ db *gorm.DB }

func NewMasterCardRepository(db *gorm.DB) MasterCardRepository { return &masterCardRepo{db: db} }

// ListByMasterCardgroup returns every master card in the group ordered by
// (position, id) so the deck order is deterministic. An empty masterCardgroupID
// short-circuits to (nil, nil): with an empty value GORM still emits the WHERE
// clause, but the empty-input guard keeps the contract explicit and mirrors the
// GORM empty-IN discipline in `.claude/rules/go-library-gotchas.md`.
func (r *masterCardRepo) ListByMasterCardgroup(ctx context.Context, masterCardgroupID string) ([]*domain.MasterCard, error) {
	if masterCardgroupID == "" {
		return nil, nil
	}
	var rows []gormMasterCard
	if err := r.db.WithContext(ctx).
		Where("master_cardgroup_id = ?", masterCardgroupID).
		Order("position ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: master card: list by master cardgroup")
	}
	out := make([]*domain.MasterCard, len(rows))
	for i := range rows {
		out[i] = masterCardToDomain(rows[i])
	}
	return out, nil
}

// FindByID returns the master card with the given primary key, or ErrNotFound
// when no row matches. Single-row PK lookup mirroring cardRepo.FindByID.
func (r *masterCardRepo) FindByID(ctx context.Context, id string) (*domain.MasterCard, error) {
	var row gormMasterCard
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: master card: find by id")
	}
	return masterCardToDomain(row), nil
}

// FindPageByMasterCardgroup paginates the master cards of a single group with
// Relay-style cursors. The window is ordered by (orderField, id); when orderBy is
// ID only `id` appears in the ORDER BY, otherwise `, id <dir>` is appended so the
// ordering is always total. Backward paging executes the query with the inverted
// direction and reverses the slice afterwards (direction-flip + ReverseSlice).
// totalCount comes from a separate COUNT(*) scoped to the group and the optional
// search filter, computed before the no-rows short-circuit so a first=0 && last=0
// request still observes the real count.
func (r *masterCardRepo) FindPageByMasterCardgroup(
	ctx context.Context,
	masterCardgroupID string,
	after, before *MasterCardCursor,
	first, last int,
	orderBy MasterCardOrderBy,
	dir SortOrder,
	search *string,
) ([]*domain.MasterCard, int64, error) {
	first = ClampPageSize(first)
	last = ClampPageSize(last)

	// Base query scoped to the master cardgroup.
	base := r.db.WithContext(ctx).Model(&gormMasterCard{}).Where("master_cards.master_cardgroup_id = ?", masterCardgroupID)

	// searchLikePattern trims, escapes LIKE metacharacters, and drops the
	// predicate when search is nil or blank, keeping blank-search handling
	// consistent across every paginated repository. The front column is citext
	// (case-insensitive); ILIKE on both columns keeps the front/back search
	// consistently case-insensitive.
	if pattern, ok := searchLikePattern(search); ok {
		base = base.Where("(master_cards.front ILIKE ? OR master_cards.back ILIKE ?)", pattern, pattern)
	}

	// totalCount from a separate COUNT(*), computed before the no-rows
	// short-circuit so callers passing first=0 still observe the real count.
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: master card: count page by master cardgroup")
	}

	if first == 0 && last == 0 {
		return []*domain.MasterCard{}, total, nil
	}

	// Backward paging executes with the inverted direction and reverses the
	// returned slice so the page boundary stays at the tail.
	effectiveDir, limit, cur, reverse := paginateSetup(dir, first, last, after, before)

	q := base.Order(masterCardOrderClause(orderBy, effectiveDir))

	if cur != nil {
		clauseStr, args, err := masterCardCursorWhere(orderBy, effectiveDir, cur)
		if err != nil {
			return nil, 0, eris.Wrap(err, "repository: master card: build cursor where")
		}
		q = q.Where(clauseStr, args...)
	}

	q = q.Limit(limit)

	var rows []gormMasterCard
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: master card: find page by master cardgroup")
	}

	if reverse {
		ReverseSlice(rows)
	}

	out := make([]*domain.MasterCard, len(rows))
	for i := range rows {
		out[i] = masterCardToDomain(rows[i])
	}
	return out, total, nil
}

// masterCardCursorSpec describes the master-card aggregate's cursor geometry.
// The primary sort column and id tie-break are prefixed with the `master_cards`
// alias used by the page query.
func masterCardCursorSpec(orderBy MasterCardOrderBy, c *MasterCardCursor) cursorSpec {
	return cursorSpec{
		alias:      "master_cards",
		orderCol:   "master_cards." + string(orderBy),
		isIDOrder:  orderBy == MasterCardOrderByID,
		fieldValue: func() (any, error) { return masterCardCursorFieldValue(orderBy, c) },
	}
}

// masterCardOrderClause renders the SQL ORDER BY tail. When orderBy is `id` only
// one column appears; otherwise the secondary `id` keeps order deterministic.
func masterCardOrderClause(orderBy MasterCardOrderBy, dir SortOrder) string {
	return buildOrderClause(masterCardCursorSpec(orderBy, nil), dir)
}

// masterCardCursorWhere builds the tuple-comparison WHERE for the supplied cursor
// and direction. ASC yields `>`, DESC yields `<`. Returns an error when the cursor
// lacks the column required by the active orderBy.
func masterCardCursorWhere(orderBy MasterCardOrderBy, dir SortOrder, c *MasterCardCursor) (string, []any, error) {
	return buildCursorWhere(masterCardCursorSpec(orderBy, c), dir, c.ID)
}

// masterCardCursorFieldValue returns the cursor value for the active orderBy
// field. An unset column is a caller bug — the usecase layer hydrates the relevant
// field before calling — so this returns an error rather than a zero value.
func masterCardCursorFieldValue(orderBy MasterCardOrderBy, c *MasterCardCursor) (any, error) {
	switch orderBy {
	case MasterCardOrderByPosition:
		if c.Position != nil {
			return *c.Position, nil
		}
	case MasterCardOrderByCreatedAt:
		if c.CreatedAt != nil {
			return *c.CreatedAt, nil
		}
	case MasterCardOrderByUpdatedAt:
		if c.UpdatedAt != nil {
			return *c.UpdatedAt, nil
		}
	}
	return nil, eris.Errorf("repository: master card: cursor missing %s column", orderBy)
}

// Create inserts a single master card. When the ID is empty a UUID v7 is
// generated via domain.NewID(); the error is propagated (no v4 fallback) per
// `.claude/rules/go-library-gotchas.md` § "`uuid.NewV7` failure must propagate".
func (r *masterCardRepo) Create(ctx context.Context, c *domain.MasterCard) error {
	if c.ID == "" {
		id, err := domain.NewID()
		if err != nil {
			return eris.Wrap(err, "repository: master card: create")
		}
		c.ID = id
	}
	if err := r.db.WithContext(ctx).Create(masterCardToRow(c)).Error; err != nil {
		if pgConstraintViolation(err, "23505", "uq_master_cards_cg_front") {
			return ErrCardDuplicateFront
		}
		if classified := classifyMasterCardFKError(err); classified != nil {
			return classified
		}
		return eris.Wrap(err, "repository: master card: create")
	}
	return nil
}

// FindByMasterCardgroupAndFront returns the master card identified by the
// (master_cardgroup_id, front) unique key, or ErrNotFound when no row matches.
// front is matched case-insensitively because the column is citext.
func (r *masterCardRepo) FindByMasterCardgroupAndFront(ctx context.Context, masterCardgroupID, front string) (*domain.MasterCard, error) {
	var row gormMasterCard
	err := r.db.WithContext(ctx).
		Where("master_cardgroup_id = ? AND front = ?", masterCardgroupID, front).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: master card: find by master cardgroup and front")
	}
	return masterCardToDomain(row), nil
}

// Update applies the field patch and returns the updated row. An all-nil patch
// short-circuits to a FindByID read so callers always receive the current row.
// A missing id surfaces as ErrNotFound. Mirrors cardRepo.Update.
func (r *masterCardRepo) Update(ctx context.Context, id string, patch MasterCardUpdate) (*domain.MasterCard, error) {
	updates := map[string]any{}
	if patch.Front != nil {
		updates["front"] = *patch.Front
	}
	if patch.Back != nil {
		updates["back"] = *patch.Back
	}
	if len(updates) == 0 {
		return r.FindByID(ctx, id)
	}

	res := r.db.WithContext(ctx).Model(&gormMasterCard{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, eris.Wrap(res.Error, "repository: master card: update")
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

// DeleteMany hard-deletes the master cards whose ids are in the list, returning
// the number of rows deleted. Empty ids short-circuits to (0, nil) so an empty
// slice can never degrade into an unbounded mass delete (GORM omits an empty
// `WHERE id IN (?)` clause; see `.claude/rules/go-library-gotchas.md` § GORM
// empty IN). There is no owner scope — master decks are admin-owned and global.
func (r *masterCardRepo) DeleteMany(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Where("id IN ?", ids).Delete(&gormMasterCard{})
	if res.Error != nil {
		return 0, eris.Wrap(res.Error, "repository: master card: delete many")
	}
	return res.RowsAffected, nil
}

// UpsertManyTx upserts master cards by (master_cardgroup_id, front). Existing
// rows have `back`, `updated_at`, and `position` overwritten. Returns the
// per-row Inserted/Updated split. Empty input is a no-op (handled by the shared
// helper). Operates on the supplied tx only.
func (r *masterCardRepo) UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.MasterCard) (UpsertManyTxResult, error) {
	rows := make([]upsertCardRow, len(cards))
	for i, c := range cards {
		rows[i] = upsertCardRow{
			ID:        c.ID,
			GroupID:   c.MasterCardgroupID,
			Front:     string(c.Front),
			Back:      string(c.Back),
			Position:  c.Position,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		}
	}
	res, err := upsertManyTx(ctx, tx, rows, "master_cards", "master_cardgroup_id")
	if err != nil {
		if classified := classifyMasterCardFKError(err); classified != nil {
			return UpsertManyTxResult{}, classified
		}
		return UpsertManyTxResult{}, eris.Wrap(err, "repository: master card: upsert many")
	}
	return res, nil
}

// ListFrontsByMasterCardgroupTx returns the sorted `front` values for the group.
func (r *masterCardRepo) ListFrontsByMasterCardgroupTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string) ([]string, error) {
	fronts, err := listFrontsByCardgroupTx(ctx, tx, masterCardgroupID, "master_cards", "master_cardgroup_id")
	if err != nil {
		return nil, eris.Wrap(err, "repository: master card: list fronts by master cardgroup")
	}
	return fronts, nil
}

// DeleteByMasterCardgroupAndFrontsTx hard-deletes master cards by the scoped
// (master_cardgroup_id, front) natural key. Empty fronts short-circuits to
// (0, nil) inside the shared helper.
func (r *masterCardRepo) DeleteByMasterCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, masterCardgroupID string, fronts []string) (int64, error) {
	affected, err := deleteByCardgroupAndFrontsTx(ctx, tx, masterCardgroupID, fronts, "master_cards", "master_cardgroup_id")
	if err != nil {
		return 0, eris.Wrap(err, "repository: master card: delete by master cardgroup and fronts")
	}
	return affected, nil
}

// Delete hard-deletes the master card by primary key. Returns ErrNotFound when
// no row matched.
func (r *masterCardRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormMasterCard{})
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: master card: delete")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func masterCardToRow(c *domain.MasterCard) *gormMasterCard {
	return &gormMasterCard{
		ID:                c.ID,
		MasterCardgroupID: c.MasterCardgroupID,
		Front:             string(c.Front),
		Back:              string(c.Back),
		CreatedAt:         c.CreatedAt,
		UpdatedAt:         c.UpdatedAt,
		Position:          c.Position,
	}
}

func masterCardToDomain(row gormMasterCard) *domain.MasterCard {
	return &domain.MasterCard{
		ID:                row.ID,
		MasterCardgroupID: row.MasterCardgroupID,
		Front:             domain.CardText(row.Front),
		Back:              domain.CardText(row.Back),
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
		Position:          row.Position,
	}
}
