package repository

import (
	"context"
	"errors"
	"fmt"
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
// the row was found, which is precisely the failure (see
// docs/backend/error-wrapping/standalone-sentinels-not-every-joins-errnotfound.md).
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

type gormCard struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid"`
	CardgroupID string    `gorm:"column:cardgroup_id"`
	Front       string    `gorm:"column:front"`
	Back        string    `gorm:"column:back"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
	Position    int       `gorm:"column:position"`
}

func (gormCard) TableName() string { return "cards" }

type CardUpdate struct {
	Front *string
	Back  *string
}

type CardReadRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	FindByIDTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error)
	FindByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error)
	ListFrontsByCardgroupTx(ctx context.Context, tx *gorm.DB, cardgroupID string) ([]string, error)
	// FindByCardgroupAndFront returns the card identified by the (cardgroup_id,
	// front) unique key, or ErrNotFound when no such row exists. The front value
	// is matched exactly; trimming is the caller's responsibility.
	FindByCardgroupAndFront(ctx context.Context, cardgroupID, front string) (*domain.Card, error)
}

type CardPageRepository interface {
	FindPageByCardgroup(
		ctx context.Context,
		cardgroupID string,
		after, before *CardCursor,
		first, last int,
		orderBy CardOrderBy,
		dir SortOrder,
		search *string,
	) (cards []*domain.Card, totalCount int64, err error)
	FindPageByCardgroupForUser(
		ctx context.Context,
		userID, cardgroupID string,
		after, before *CardCursor,
		first, last int,
		orderBy CardOrderBy,
		dir SortOrder,
		search *string,
	) (cards []*domain.Card, totalCount int64, err error)
	FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, now, reviewedBefore time.Time, limit int) ([]domain.DueCard, error)
	// FindPracticeCardsForUser returns the FSRS-safe practice pool: cards the
	// user already reviewed at or after reviewedAfter (the start-of-day cutoff).
	// This is the inverse window of FindDueCardsForUser's review window — it
	// consults last_review but not due, and never advances FSRS scheduling.
	FindPracticeCardsForUser(ctx context.Context, userID, cardgroupID string, reviewedAfter time.Time, limit int) ([]domain.DueCard, error)
}

type CardWriteRepository interface {
	Create(ctx context.Context, card *domain.Card) error
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
	// DeleteByCardgroupAndFrontsTx hard-deletes cards by the scoped
	// (cardgroup_id, front) natural key. Scoping is by cardgroup_id only —
	// callers must verify the cardgroup is reachable by the calling owner
	// before invoking this method (NotionSyncUsecase is the canonical caller).
	//
	// Empty fronts short-circuits to (0, nil) without touching the DB. With an
	// empty slice GORM v2 omits the `WHERE front IN (?)` clause altogether,
	// which would convert this `Delete` into a delete-all-cards-in-cardgroup.
	// See `.claude/rules/go-library-gotchas.md` § GORM empty IN.
	DeleteByCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error)
	// UpsertManyTx upserts cards by (cardgroup_id, front). Existing rows have
	// their `back`, `updated_at`, and `position` columns overwritten. Returns the
	// per-row split between Inserted and Updated. Empty input is a no-op.
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (UpsertManyTxResult, error)
}

type CardRepository interface {
	CardReadRepository
	CardPageRepository
	CardWriteRepository
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

func (r *cardRepo) ListFrontsByCardgroupTx(ctx context.Context, tx *gorm.DB, cardgroupID string) ([]string, error) {
	var fronts []string
	if err := tx.WithContext(ctx).
		Model(&gormCard{}).
		Where("cardgroup_id = ?", cardgroupID).
		Order("front ASC").
		Pluck("front", &fronts).Error; err != nil {
		return nil, eris.Wrap(err, "repository: list card fronts by cardgroup")
	}
	return fronts, nil
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
	return r.FindPageByCardgroupForUser(ctx, "", cardgroupID, after, before, first, last, orderBy, dir, search)
}

func (r *cardRepo) FindPageByCardgroupForUser(
	ctx context.Context,
	userID, cardgroupID string,
	after, before *CardCursor,
	first, last int,
	orderBy CardOrderBy,
	dir SortOrder,
	search *string,
) ([]*domain.Card, int64, error) {
	userID = coalesceUserIDForJoin(userID)
	first = ClampPageSize(first)
	last = ClampPageSize(last)

	// Base query scoped to the cardgroup.
	base := r.db.WithContext(ctx).Model(&gormCard{}).Where("cards.cardgroup_id = ?", cardgroupID)

	// Non-nil search is guaranteed by the usecase to be non-empty and trimmed.
	// escapeLikePattern guards against LIKE metacharacter injection.
	if search != nil {
		pattern := "%" + escapeLikePattern(*search) + "%"
		base = base.Where("(cards.front ILIKE ? OR cards.back ILIKE ?)", pattern, pattern)
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
		effectiveDir = InvertDir(dir)
		limit = last
		cursor = before
		reverse = true
	}

	q := base
	if orderBy == CardOrderByDue {
		q = q.Select("cards.*").
			Joins("LEFT JOIN user_card_fsrs ucs ON ucs.user_id = ? AND ucs.card_id = cards.id", userID).
			Order(orderClause(orderBy, effectiveDir))
	} else {
		q = q.Order(orderClause(orderBy, effectiveDir))
	}

	if cursor != nil {
		clauseStr, args, err := cursorWhere(orderBy, effectiveDir, cursor)
		if err != nil {
			return nil, 0, eris.Wrap(err, "repository: build cursor where")
		}
		q = q.Where(clauseStr, args...)
	}

	q = q.Limit(limit)

	var rows []gormCard
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: find page by cardgroup")
	}

	if reverse {
		ReverseSlice(rows)
	}

	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, total, nil
}

// orderClause renders the SQL ORDER BY tail. When orderBy is `id` only one
// column appears; otherwise the secondary `id` keeps order deterministic.
func orderClause(orderBy CardOrderBy, dir SortOrder) string {
	d := string(dir)
	if orderBy == CardOrderByID {
		return "cards.id " + d
	}
	if orderBy == CardOrderByDue {
		return "COALESCE(ucs.due, cards.created_at) " + d + ", cards.id " + d
	}
	return "cards." + string(orderBy) + " " + d + ", cards.id " + d
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
		return "cards.id " + op + " ?", []any{c.ID}, nil
	}
	field := "cards." + string(orderBy)
	if orderBy == CardOrderByDue {
		field = "COALESCE(ucs.due, cards.created_at)"
	}
	val, err := cursorFieldValue(orderBy, c)
	if err != nil {
		return "", nil, err
	}
	// Tuple compare: (field, id) op (val, c.ID).
	return "(" + field + " " + op + " ? OR (" + field + " = ? AND cards.id " + op + " ?))",
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

func (r *cardRepo) FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, now, reviewedBefore time.Time, limit int) ([]domain.DueCard, error) {
	return findDueCardsOn(r.db.WithContext(ctx), userID, cardgroupID, now, reviewedBefore, limit)
}

func (r *cardRepo) FindPracticeCardsForUser(ctx context.Context, userID, cardgroupID string, reviewedAfter time.Time, limit int) ([]domain.DueCard, error) {
	return findPracticeCardsOn(r.db.WithContext(ctx), userID, cardgroupID, reviewedAfter, limit)
}

// dueCardRow is the raw scan target for findDueCardsOn. It holds all cards.*
// columns as flat fields plus nullable FSRS columns from the LEFT JOIN.
// Embedding gormCard is intentionally avoided: gormCard carries a TableName()
// method that confuses GORM's embedded-struct schema parser when the outer
// scan target is a different type.
type dueCardRow struct {
	ID          string     `gorm:"column:id"`
	CardgroupID string     `gorm:"column:cardgroup_id"`
	Front       string     `gorm:"column:front"`
	Back        string     `gorm:"column:back"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
	Position    int        `gorm:"column:position"`
	State       *int       `gorm:"column:state"`
	Due         *time.Time `gorm:"column:due"`
}

// Learn window:    due IS NOT NULL AND due <= now AND last_review < boundary.
// Practice window: last_review >= boundary; due not consulted.
// Both usecase methods (NextDueCards and PracticeTodaysCards) derive the
// boundary from the same startOfDayJST formula, computed once per call
// before hitting the repository. Changing the formula or comparator for
// one window without the other makes a card vanish from (or appear in)
// both queues.
//
// findDueCardsOn fetches the cards eligible for a learning session in two
// independent LIMIT windows and concatenates them: review cards first, then
// new (never-reviewed) cards. Splitting the fetch is what keeps a large
// new-card backlog from evicting due reviews under a single LIMIT. The
// returned slice may hold up to 2*limit rows; OrderingPolicy (in the
// usecase) applies the final interleave and truncation per session.
//
// Review window: due has arrived AND the card was last reviewed before
// reviewedBefore (the caller's local start-of-today) — a card swiped today
// never re-enters today's queue. Learning-phase rows (latest rating
// Again/Hard) outrank Review-state rows; random() varies the selection
// inside each phase per session. The phase-first ORDER is a contract with
// OrderingPolicy's shuffleWithinPhase.
//
// New window: no FSRS row yet; random() samples uniformly across the whole
// unseen pool so consecutive sessions surface different cards instead of
// walking the deterministic created_at/position (document) order.
func findDueCardsOn(db *gorm.DB, userID, cardgroupID string, now, reviewedBefore time.Time, limit int) ([]domain.DueCard, error) {
	userID = coalesceUserIDForJoin(userID)
	if limit <= 0 {
		return []domain.DueCard{}, nil
	}

	reviewOrder := fmt.Sprintf(
		"CASE WHEN ucs.state IN (%d, %d) THEN 0 ELSE 1 END, random()",
		domain.FSRSStateLearning, domain.FSRSStateRelearning,
	)
	reviewRows, err := dueRowsOn(db, userID,
		"cards.cardgroup_id = ? AND ucs.due IS NOT NULL AND ucs.due <= ? AND ucs.last_review < ?",
		[]any{cardgroupID, now, reviewedBefore},
		reviewOrder,
		limit,
		"repository: card: find due cards")
	if err != nil {
		return nil, err
	}

	newRows, err := dueRowsOn(db, userID,
		"cards.cardgroup_id = ? AND ucs.due IS NULL",
		[]any{cardgroupID},
		"random()",
		limit,
		"repository: card: find due cards")
	if err != nil {
		return nil, err
	}

	out := make([]domain.DueCard, 0, len(reviewRows)+len(newRows))
	for _, rows := range [][]dueCardRow{reviewRows, newRows} {
		mapped, err := dueCardsFromRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, mapped...)
	}
	return out, nil
}

// findPracticeCardsOn fetches the FSRS-safe practice pool: cards the user
// already reviewed at or after the boundary (the same startOfDayJST cutoff the
// learn window uses). This is the INVERSE window of findDueCardsOn's review
// window — practice consults last_review but not due, and uses >= where learn
// uses <. random() gives a fresh arrangement per practice round.
//
// NULL last_review (never-reviewed cards) can never satisfy `>=`, so no
// `IS NOT NULL` guard is needed.
func findPracticeCardsOn(db *gorm.DB, userID, cardgroupID string, reviewedAfter time.Time, limit int) ([]domain.DueCard, error) {
	userID = coalesceUserIDForJoin(userID)
	if limit <= 0 {
		return []domain.DueCard{}, nil
	}

	rows, err := dueRowsOn(db, userID,
		"cards.cardgroup_id = ? AND ucs.last_review >= ?",
		[]any{cardgroupID, reviewedAfter},
		"random()",
		limit,
		"repository: card: find practice cards")
	if err != nil {
		return nil, err
	}
	return dueCardsFromRows(rows)
}

// dueRowsOn runs the cards-with-FSRS LEFT JOIN scoped to userID with the given
// WHERE predicate, ORDER BY clause, and LIMIT. Shared by the review and new-card
// fetches in findDueCardsOn and the practice fetch in findPracticeCardsOn so the
// SELECT/JOIN never drift between them. wrapMsg is supplied by the caller because
// a shared helper must not embed a caller-specific layer prefix.
func dueRowsOn(db *gorm.DB, userID, where string, whereArgs []any, order string, limit int, wrapMsg string) ([]dueCardRow, error) {
	var rows []dueCardRow
	// ucs.last_review is used in WHERE clauses by both callers (findDueCardsOn
	// review window and findPracticeCardsOn) but is not projected into
	// dueCardRow — it is filter-only and not needed after scan.
	if err := db.
		Table("cards").
		Select("cards.id, cards.cardgroup_id, cards.front, cards.back, cards.created_at, cards.updated_at, cards.position, ucs.state, ucs.due").
		Joins("LEFT JOIN user_card_fsrs ucs ON ucs.user_id = ? AND ucs.card_id = cards.id", userID).
		Where(where, whereArgs...).
		Order(order).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, wrapMsg)
	}
	return rows, nil
}

// dueCardsFromRows maps raw dueCardRow scan results into domain.DueCard values,
// defaulting State to FSRSStateNew and Due to created_at when the LEFT JOIN
// produced NULL FSRS columns (a new card). Shared by both fetches in
// findDueCardsOn.
func dueCardsFromRows(rows []dueCardRow) ([]domain.DueCard, error) {
	out := make([]domain.DueCard, len(rows))
	for i, r := range rows {
		c := &domain.Card{
			ID:          r.ID,
			CardgroupID: r.CardgroupID,
			Front:       domain.CardText(r.Front),
			Back:        domain.CardText(r.Back),
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
			Position:    r.Position,
		}
		dc := domain.DueCard{Card: c, State: domain.FSRSStateNew, Due: r.CreatedAt}
		if r.State != nil {
			s := domain.FSRSCardState(*r.State)
			if !s.IsValid() {
				return nil, eris.Errorf("repository: card: invalid FSRSCardState %d", *r.State)
			}
			dc.State = s
		}
		if r.Due != nil {
			dc.Due = *r.Due
		}
		out[i] = dc
	}
	return out, nil
}

func coalesceUserIDForJoin(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return "00000000-0000-0000-0000-000000000000"
	}
	return userID
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
// `back`, `updated_at`, and `position` overwritten. The conflict key requires the
// unique index `uq_cards_cardgroup_front` (migration 20260503000000).
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

	// Build a single multi-row INSERT. Each card contributes 7 placeholders
	// matching the column list below.
	const columns = `(id, cardgroup_id, front, back, created_at, updated_at, position)`
	const rowPH = "(?, ?, ?, ?, ?, ?, ?)"

	var sb strings.Builder
	sb.WriteString("INSERT INTO cards ")
	sb.WriteString(columns)
	sb.WriteString(" VALUES ")
	args := make([]any, 0, len(cards)*7)
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
			c.CreatedAt,
			c.UpdatedAt,
			c.Position,
		)
	}
	sb.WriteString(`
        ON CONFLICT (cardgroup_id, front)
        DO UPDATE SET back = EXCLUDED.back, updated_at = now(), position = EXCLUDED.position
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

func (r *cardRepo) DeleteByCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error) {
	if len(fronts) == 0 {
		return 0, nil
	}
	res := tx.WithContext(ctx).
		Where("cardgroup_id = ? AND front IN ?", cardgroupID, fronts).
		Delete(&gormCard{})
	if res.Error != nil {
		return 0, eris.Wrap(res.Error, "repository: delete cards by cardgroup and fronts")
	}
	return res.RowsAffected, nil
}

func cardToRow(card *domain.Card) *gormCard {
	return &gormCard{
		ID:          card.ID,
		CardgroupID: card.CardgroupID,
		Front:       string(card.Front),
		Back:        string(card.Back),
		CreatedAt:   card.CreatedAt,
		UpdatedAt:   card.UpdatedAt,
		Position:    card.Position,
	}
}

func cardToDomain(row gormCard) *domain.Card {
	return &domain.Card{
		ID:          row.ID,
		CardgroupID: row.CardgroupID,
		Front:       domain.CardText(row.Front),
		Back:        domain.CardText(row.Back),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		Position:    row.Position,
	}
}
