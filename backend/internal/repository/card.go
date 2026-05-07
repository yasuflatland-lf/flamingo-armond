package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"backend/internal/domain"
)

// ErrCardDuplicateFront is returned by Create when an INSERT collides with the
// (cardgroup_id, front) unique index. Standalone — do NOT join with ErrNotFound;
// the row was found, which is precisely the failure (see error-wrapping.md).
var ErrCardDuplicateFront = errors.New("repository: card with same front exists in cardgroup")

// CardOrderBy is the allowlist of fields that paginated card queries may sort by.
// Lexicographic tuple order is always (orderField, id) so cursors stay deterministic
// even when the order field has duplicate values.
type CardOrderBy string

const (
	CardOrderByID        CardOrderBy = "id"
	CardOrderByCreatedAt CardOrderBy = "created_at"
	CardOrderByUpdatedAt CardOrderBy = "updated_at"
	CardOrderByDue       CardOrderBy = "due"
)

// SortOrder mirrors the GraphQL SortOrder enum.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// CardCursor is an opaque cursor for paginated card queries.
// Only the fields relevant to the active OrderBy need to be populated.
// ID is always populated and acts as the secondary key in the tuple comparison.
type CardCursor struct {
	ID        string
	Due       *time.Time
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

// pageCap is the upper bound for first/last in paginated queries. Set to
// usecase.maxPageSize+1 so the usecase's "+1 fetch" trick can detect another
// page even when the caller asks for the documented max (100).
const pageCap = 101

type gormCard struct {
	ID            string    `gorm:"column:id;primaryKey;type:uuid"`
	CardgroupID   string    `gorm:"column:cardgroup_id"`
	Front         string    `gorm:"column:front"`
	Back          string    `gorm:"column:back"`
	Due           time.Time `gorm:"column:due"`
	Stability     float64   `gorm:"column:stability"`
	Difficulty    float64   `gorm:"column:difficulty"`
	ElapsedDays   int       `gorm:"column:elapsed_days"`
	ScheduledDays int       `gorm:"column:scheduled_days"`
	Reps          int       `gorm:"column:reps"`
	Lapses        int       `gorm:"column:lapses"`
	State         int       `gorm:"column:state"`
	LastReview    time.Time `gorm:"column:last_review"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (gormCard) TableName() string { return "cards" }

type CardUpdate struct {
	Front *string
	Back  *string
}

type CardRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	FindByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error)
	FindByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error)
	FindPageByCardgroup(
		ctx context.Context,
		cardgroupID string,
		after, before *CardCursor,
		first, last int,
		orderBy CardOrderBy,
		dir SortOrder,
		search *string,
	) (cards []*domain.Card, totalCount int64, err error)
	FindDueCardsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error)
	Create(ctx context.Context, card *domain.Card) error
	// FindByCardgroupAndFront returns the card identified by the (cardgroup_id,
	// front) unique key, or ErrNotFound when no such row exists. The front value
	// is matched exactly; trimming is the caller's responsibility.
	FindByCardgroupAndFront(ctx context.Context, cardgroupID, front string) (*domain.Card, error)
	UpdateFSRSStateTx(ctx context.Context, tx *gorm.DB, id string, state domain.FSRSState) error
	Update(ctx context.Context, id string, patch CardUpdate) (*domain.Card, error)
	Delete(ctx context.Context, id string) error
	// DeleteByIDsTx hard-deletes the cards whose ids are in the list AND whose
	// cardgroup is owned by ownerID. Returns the number of rows actually deleted
	// (cards owned by other users are silently skipped at SQL level so a single
	// foreign id in the list does not abort the batch).
	//
	// Empty ids short-circuits to (0, nil) without touching the DB. With an empty
	// slice GORM v2 omits the `WHERE id IN (?)` clause altogether, which would
	// convert this `Delete` into an unbounded mass delete — far worse than a slow scan.
	DeleteByIDsTx(ctx context.Context, tx *gorm.DB, ownerID string, ids []string) (int64, error)
	// UpsertManyTx upserts cards by (cardgroup_id, front). Existing rows have
	// their `back` and `updated_at` columns overwritten; new rows are inserted
	// using the FSRS state values supplied on each domain.Card. Returns the
	// per-row split between Inserted and Updated. Empty input is a no-op.
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (UpsertManyTxResult, error)
}

type cardRepo struct{ db *gorm.DB }

func NewCardRepository(db *gorm.DB) CardRepository { return &cardRepo{db: db} }

func (r *cardRepo) FindByID(ctx context.Context, id string) (*domain.Card, error) {
	return findCardByID(ctx, r.db, id)
}

func (r *cardRepo) FindByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error) {
	return findCardByID(ctx, tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
}

func findCardByID(ctx context.Context, db *gorm.DB, id string) (*domain.Card, error) {
	var row gormCard
	err := db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: find card by id")
	}
	return cardToDomain(row), nil
}

func (r *cardRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error) {
	if len(ids) == 0 {
		return map[string]*domain.Card{}, nil
	}
	var rows []gormCard
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find cards by ids")
	}
	out := make(map[string]*domain.Card, len(rows))
	for i := range rows {
		card := cardToDomain(rows[i])
		out[card.ID] = card
	}
	return out, nil
}

func (r *cardRepo) FindByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error) {
	var rows []gormCard
	if err := r.db.WithContext(ctx).
		Where("cardgroup_id = ?", cardgroupID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find cards by cardgroup")
	}
	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, nil
}

// escapeLike escapes the three Postgres LIKE metacharacters so user-supplied
// text is treated as a literal substring. Order matters: escape '\' first,
// otherwise the second pass would re-escape the already-escaped sequences.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// FindPageByCardgroup returns a window of cards for a cardgroup ordered by
// (orderField, id) so cursors stay deterministic. Forward paging uses `after`
// + `first`; backward paging uses `before` + `last`. totalCount reflects every
// row in the cardgroup, not just the page.
// When search is non-nil and non-empty, only cards whose front OR back contains
// the search text (case-insensitive ILIKE partial match) are returned.
// The search is applied to both totalCount and the page window.
func (r *cardRepo) FindPageByCardgroup(
	ctx context.Context,
	cardgroupID string,
	after, before *CardCursor,
	first, last int,
	orderBy CardOrderBy,
	dir SortOrder,
	search *string,
) ([]*domain.Card, int64, error) {
	first = clampPageSize(first)
	last = clampPageSize(last)

	// Base query scoped to the cardgroup.
	base := r.db.WithContext(ctx).Model(&gormCard{}).Where("cardgroup_id = ?", cardgroupID)

	// Apply the search filter when search is non-nil. The usecase layer
	// guarantees that a non-nil pointer holds a non-empty, trimmed string
	// (whitespace-only inputs are normalized to nil before reaching here).
	// escapeLike prevents LIKE metacharacter injection.
	if search != nil {
		pattern := "%" + escapeLike(*search) + "%"
		base = base.Where("(front ILIKE ? OR back ILIKE ?)", pattern, pattern)
	}

	// totalCount comes from a separate COUNT(*) scoped to the cardgroup (and
	// search filter, if active). Computed before the no-rows short-circuit so
	// callers passing first=0 still observe the real count.
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: count cards by cardgroup")
	}

	if first == 0 && last == 0 {
		return []*domain.Card{}, total, nil
	}

	// Backward paging executes the query with the inverted direction and
	// reverses the slice afterwards.
	effectiveDir := dir
	limit := first
	cursor := after
	reverse := false
	if last > 0 {
		effectiveDir = invertDir(dir)
		limit = last
		cursor = before
		reverse = true
	}

	q := base

	if cursor != nil {
		clauseStr, args, err := cursorWhere(orderBy, effectiveDir, cursor)
		if err != nil {
			return nil, 0, eris.Wrap(err, "repository: build cursor where")
		}
		q = q.Where(clauseStr, args...)
	}

	q = q.Order(orderClause(orderBy, effectiveDir)).Limit(limit)

	var rows []gormCard
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: find page by cardgroup")
	}

	if reverse {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}

	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, total, nil
}

func clampPageSize(n int) int {
	if n < 0 {
		return 0
	}
	if n > pageCap {
		return pageCap
	}
	return n
}

func invertDir(d SortOrder) SortOrder {
	if d == SortDesc {
		return SortAsc
	}
	return SortDesc
}

// orderClause renders the SQL ORDER BY tail. When orderBy is `id` only one
// column appears; otherwise the secondary `id` keeps order deterministic.
func orderClause(orderBy CardOrderBy, dir SortOrder) string {
	d := string(dir)
	if orderBy == CardOrderByID {
		return "id " + d
	}
	return string(orderBy) + " " + d + ", id " + d
}

// cursorWhere builds the tuple-comparison WHERE for the supplied cursor and
// direction. ASC yields `>`, DESC yields `<`. Returns an error when the
// cursor lacks the column required by the active orderBy.
func cursorWhere(orderBy CardOrderBy, dir SortOrder, c *CardCursor) (string, []any, error) {
	op := ">"
	if dir == SortDesc {
		op = "<"
	}
	if orderBy == CardOrderByID {
		return "id " + op + " ?", []any{c.ID}, nil
	}
	field := string(orderBy)
	val, err := cursorFieldValue(orderBy, c)
	if err != nil {
		return "", nil, err
	}
	// Tuple compare: (field, id) op (val, c.ID).
	return "(" + field + " " + op + " ? OR (" + field + " = ? AND id " + op + " ?))",
		[]any{val, val, c.ID}, nil
}

// cursorFieldValue returns the cursor value for the active orderBy field.
// An unset column is a caller bug — the usecase layer hydrates the relevant
// field before calling — so this returns an error rather than a zero value.
func cursorFieldValue(orderBy CardOrderBy, c *CardCursor) (any, error) {
	switch orderBy {
	case CardOrderByDue:
		if c.Due != nil {
			return *c.Due, nil
		}
	case CardOrderByCreatedAt:
		if c.CreatedAt != nil {
			return *c.CreatedAt, nil
		}
	case CardOrderByUpdatedAt:
		if c.UpdatedAt != nil {
			return *c.UpdatedAt, nil
		}
	}
	return nil, eris.Errorf("cursor missing %s column", orderBy)
}

func (r *cardRepo) FindDueCardsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error) {
	if limit <= 0 {
		return []*domain.Card{}, nil
	}
	var rows []gormCard
	if err := tx.WithContext(ctx).
		Where("cardgroup_id = ? AND due <= ?", cardgroupID, now).
		Order("due ASC, id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: find due cards")
	}
	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, nil
}

func (r *cardRepo) Create(ctx context.Context, card *domain.Card) error {
	if err := r.db.WithContext(ctx).Create(cardToRow(card)).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			strings.Contains(pgErr.ConstraintName, "uq_cards_cardgroup_front") {
			return ErrCardDuplicateFront
		}
		return eris.Wrap(err, "repository: create card")
	}
	return nil
}

func (r *cardRepo) FindByCardgroupAndFront(ctx context.Context, cardgroupID, front string) (*domain.Card, error) {
	var row gormCard
	err := r.db.WithContext(ctx).
		Where("cardgroup_id = ? AND front = ?", cardgroupID, front).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: find card by cardgroup and front")
	}
	return cardToDomain(row), nil
}

func (r *cardRepo) UpdateFSRSStateTx(ctx context.Context, tx *gorm.DB, id string, state domain.FSRSState) error {
	updates := map[string]any{
		"due":            state.Due,
		"stability":      state.Stability,
		"difficulty":     state.Difficulty,
		"elapsed_days":   state.ElapsedDays,
		"scheduled_days": state.ScheduledDays,
		"reps":           state.Reps,
		"lapses":         state.Lapses,
		"state":          int(state.State),
		"last_review":    state.LastReview,
	}
	res := tx.WithContext(ctx).Model(&gormCard{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: update card fsrs state")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *cardRepo) Update(ctx context.Context, id string, patch CardUpdate) (*domain.Card, error) {
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

	res := r.db.WithContext(ctx).Model(&gormCard{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, eris.Wrap(res.Error, "repository: update card")
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.FindByID(ctx, id)
}

func (r *cardRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormCard{})
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: delete card")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpsertManyTxResult counts the outcome of an UpsertManyTx call.
// Inserted+Updated equals len(input cards) for a successful call.
type UpsertManyTxResult struct {
	Inserted int64
	Updated  int64
}

// UpsertManyTx upserts cards by (cardgroup_id, front). Existing rows have
// `back` and `updated_at` overwritten; new rows are inserted with the supplied
// FSRS state values. The conflict key requires the unique index
// `uq_cards_cardgroup_front` (migration 20260503000000).
//
// Counts are derived per-row from the PostgreSQL system column `xmax`. A
// freshly inserted row has `xmax = 0` in the same transaction; a row updated
// via `ON CONFLICT DO UPDATE` has `xmax` set to the current transaction id.
// The RETURNING clause exposes `xmax = 0 AS inserted` so the split can be
// computed without a second query.
//
// The method is transaction-safe: it operates on the supplied tx only and
// never reaches back to r.db. Empty input returns a zero-valued result and
// no error.
func (r *cardRepo) UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (UpsertManyTxResult, error) {
	if len(cards) == 0 {
		return UpsertManyTxResult{}, nil
	}

	// Pre-fill any missing IDs so the RETURNING clause classifies every row
	// the caller handed us. UUID v7 is the project-wide convention; v4
	// fallback is rejected per .claude/rules/go-library-gotchas.md.
	for _, c := range cards {
		if strings.TrimSpace(c.ID) == "" {
			id, err := uuid.NewV7()
			if err != nil {
				return UpsertManyTxResult{}, eris.Wrap(err, "repository: upsert many cards: uuid v7")
			}
			c.ID = id.String()
		}
	}

	// Build a single multi-row INSERT. Each card contributes 15 placeholders
	// matching the column list below.
	const columns = `(id, cardgroup_id, front, back, due, stability, difficulty,
                    elapsed_days, scheduled_days, reps, lapses, state, last_review,
                    created_at, updated_at)`
	const rowPH = "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"

	var sb strings.Builder
	sb.WriteString("INSERT INTO cards ")
	sb.WriteString(columns)
	sb.WriteString(" VALUES ")
	args := make([]any, 0, len(cards)*15)
	for i, c := range cards {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(rowPH)
		args = append(args,
			c.ID,
			c.CardgroupID,
			c.Front,
			c.Back,
			c.FSRS.Due,
			c.FSRS.Stability,
			c.FSRS.Difficulty,
			c.FSRS.ElapsedDays,
			c.FSRS.ScheduledDays,
			c.FSRS.Reps,
			c.FSRS.Lapses,
			int(c.FSRS.State),
			c.FSRS.LastReview,
			c.CreatedAt,
			c.UpdatedAt,
		)
	}
	sb.WriteString(`
        ON CONFLICT (cardgroup_id, front)
        DO UPDATE SET back = EXCLUDED.back, updated_at = now()
        RETURNING (xmax = 0) AS inserted`)

	type returnedRow struct {
		Inserted bool `gorm:"column:inserted"`
	}
	var rows []returnedRow
	if err := tx.WithContext(ctx).Raw(sb.String(), args...).Scan(&rows).Error; err != nil {
		return UpsertManyTxResult{}, eris.Wrap(err, "repository: upsert many cards")
	}

	if len(rows) != len(cards) {
		return UpsertManyTxResult{}, eris.Errorf(
			"repository: upsert many cards: returned %d rows, expected %d",
			len(rows), len(cards),
		)
	}

	var res UpsertManyTxResult
	for _, r := range rows {
		if r.Inserted {
			res.Inserted++
		} else {
			res.Updated++
		}
	}
	return res, nil
}

func (r *cardRepo) DeleteByIDsTx(ctx context.Context, tx *gorm.DB, ownerID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// Owner check at SQL: cards.cardgroup_id must reference a cardgroup the
	// user owns. The subselect is the SOLE ownership gate — the usecase does
	// no read-side owner check, so foreign-owned ids in the list are silently
	// filtered out here. Do not remove the cardgroup_id IN (...) clause
	// without adding an equivalent guard upstream.
	res := tx.WithContext(ctx).
		Where("id IN ? AND cardgroup_id IN (?)", ids,
			tx.Model(&gormCardgroup{}).Select("id").Where("owner_id = ?", ownerID),
		).
		Delete(&gormCard{})
	if res.Error != nil {
		return 0, eris.Wrap(res.Error, "repository: bulk delete cards")
	}
	return res.RowsAffected, nil
}

func cardToRow(card *domain.Card) *gormCard {
	return &gormCard{
		ID:            card.ID,
		CardgroupID:   card.CardgroupID,
		Front:         card.Front,
		Back:          card.Back,
		Due:           card.FSRS.Due,
		Stability:     card.FSRS.Stability,
		Difficulty:    card.FSRS.Difficulty,
		ElapsedDays:   card.FSRS.ElapsedDays,
		ScheduledDays: card.FSRS.ScheduledDays,
		Reps:          card.FSRS.Reps,
		Lapses:        card.FSRS.Lapses,
		State:         int(card.FSRS.State),
		LastReview:    card.FSRS.LastReview,
		CreatedAt:     card.CreatedAt,
		UpdatedAt:     card.UpdatedAt,
	}
}

func cardToDomain(row gormCard) *domain.Card {
	return &domain.Card{
		ID:          row.ID,
		CardgroupID: row.CardgroupID,
		Front:       row.Front,
		Back:        row.Back,
		FSRS: domain.FSRSState{
			Due:           row.Due,
			Stability:     row.Stability,
			Difficulty:    row.Difficulty,
			ElapsedDays:   row.ElapsedDays,
			ScheduledDays: row.ScheduledDays,
			Reps:          row.Reps,
			Lapses:        row.Lapses,
			State:         domain.FSRSCardState(row.State),
			LastReview:    row.LastReview,
		},
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
