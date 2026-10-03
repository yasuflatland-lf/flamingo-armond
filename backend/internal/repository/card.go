package repository

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"backend/internal/domain"
)

// cardsFrontIndex is the unique (cardgroup_id, front) index on public.cards.
const cardsFrontIndex = "uq_cards_cardgroup_front"

// ErrCardDuplicateFront is returned by Create, Update and FoldFrontCaseToTx when the
// write collides with the (cardgroup_id, front) unique index. Standalone — do NOT join with
// ErrNotFound; the row was found, which is precisely the failure (see
// docs/backend/error-wrapping/standalone-sentinels-not-every-joins-errnotfound.md).
var ErrCardDuplicateFront = errors.New("repository: card with same front exists in cardgroup")

// classifyCardDuplicateFront maps a Postgres unique violation on the
// (cardgroup_id, front) index to ErrCardDuplicateFront, and returns nil for any
// other error. Every write path that can collide — INSERT (Create), UPDATE (Update)
// and the merge case fold (FoldFrontCaseToTx) — can hit the same constraint, so they
// share this classifier rather than each spelling out the code/constraint pair.
func classifyCardDuplicateFront(err error) error {
	if pgConstraintViolation(err, "23505", cardsFrontIndex) {
		return ErrCardDuplicateFront
	}
	return nil
}

// ErrCardCardgroupNotFound is returned by Create / UpsertManyTx when the
// target cardgroup_id does not exist — a Postgres FK violation (23503) on
// cards_cardgroup_id_fkey. Joined with ErrNotFound so callers matching the
// general sentinel keep working. Named with the Card aggregate prefix because
// ErrCardgroupNotFound is owned by the user-preference aggregate.
var ErrCardCardgroupNotFound = errors.Join(
	errors.New("repository: card: cardgroup not found"),
	ErrNotFound,
)

// classifyCardFKError maps a Postgres FK violation (code 23503) on the
// cards.cardgroup_id foreign key to ErrCardCardgroupNotFound. Returns nil for
// any other error so callers can use it as a pre-filter before falling through
// to eris.Wrap (mirrors classifyMasterCardFKError).
func classifyCardFKError(err error) error {
	if pgConstraintViolation(err, "23503", "cardgroup_id") {
		return ErrCardCardgroupNotFound
	}
	return nil
}

type gormCard struct {
	ID          string    `gorm:"column:id;primaryKey;type:uuid"`
	CardgroupID string    `gorm:"column:cardgroup_id"`
	Front       string    `gorm:"column:front"`
	Back        string    `gorm:"column:back"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;->"`
	Position    int       `gorm:"column:position"`
	// OrderKey carries the value the page query ORDERED BY, projected into the
	// same result set. It backs no column on `cards`: the `->` tag makes it
	// read-only so GORM never tries to write or migrate it, and it stays nil on
	// every query that does not alias a column `order_key`.
	//
	// It exists for the DUE ordering, whose key is `COALESCE(ucs.due,
	// cards.created_at)` over a LEFT JOIN and therefore appears on no card
	// column. Recovering that value with a second query would read a different
	// snapshot than the one that ordered the page, so a concurrent review of the
	// boundary card would mint a cursor keyed to a position the page never used.
	// Selecting it alongside the row keeps emit and order on one snapshot.
	OrderKey *time.Time `gorm:"->;column:order_key"`
}

func (gormCard) TableName() string { return "cards" }

type CardUpdate struct {
	Front *string
	Back  *string
}

type CardReadRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Card, error)
	FindByIDForUpdateTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error)
	// FindByCardgroupAndFront returns the card identified by the (cardgroup_id,
	// front) unique key, or ErrNotFound when no such row exists. The front value
	// is matched exactly; trimming is the caller's responsibility.
	FindByCardgroupAndFront(ctx context.Context, cardgroupID, front string) (*domain.Card, error)
	// CountMatchingFrontsFold returns the number of distinct case-folded fronts
	// in the cardgroup that match the caller-supplied lowercase fronts. It counts
	// multiple stored case variants once. Empty fronts returns 0 without a query.
	// Inputs above bulkStatementChunkRows distinct fronts run as several pooled
	// statements, each on its own snapshot.
	CountMatchingFrontsFold(ctx context.Context, cardgroupID string, loweredFronts []string) (int64, error)
}

type CardPageRepository interface {
	// FindPageByCardgroupForUser pages a cardgroup's cards forward by (orderBy, id) from `after`,
	// returning at most `first` rows. A non-blank search (ILIKE on front/back) filters both the window and
	// totalCount. userID only picks the viewer's user_card_fsrs row for DUE ordering; it is not an
	// ownership check. orderKeys maps card id to the ORDER BY value read in the same query (nil for ID),
	// so v2 cursors never re-read it from a later snapshot.
	FindPageByCardgroupForUser(
		ctx context.Context,
		userID, cardgroupID string,
		after *CardCursor,
		first int,
		orderBy CardOrderBy,
		dir SortOrder,
		search *string,
	) (cards []*domain.Card, totalCount int64, orderKeys map[string]time.Time, err error)
}

// CardSessionRepository reads a learn/practice session's card pool. Unlike
// CardPageRepository these are non-paginated, limit-capped session fetches:
// no cursor, no orderBy, no totalCount.
type CardSessionRepository interface {
	// FindDueCardsForUser returns review cards due before window.DueBefore,
	// highest FSRS retrievability first, then never-seen cards newest-added first.
	FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, window domain.LearnWindow, limit int) ([]domain.DueCard, error)
	// FindPracticeCardsForUser returns the FSRS-safe practice pool: cards whose
	// last_review is at or after reviewedAfter. Callers pass
	// LearnWindow.PracticeReviewedAfter(), so practice is the exact complement of
	// FindDueCardsForUser's last_review guards (due is not consulted); it never
	// advances FSRS scheduling.
	FindPracticeCardsForUser(ctx context.Context, userID, cardgroupID string, reviewedAfter time.Time, limit int) ([]domain.DueCard, error)
}

type CardWriteRepository interface {
	Create(ctx context.Context, card *domain.Card) error
	Update(ctx context.Context, id string, patch CardUpdate) (*domain.Card, error)
	Delete(ctx context.Context, id string) error
	// DeleteByIDs hard-deletes the cards whose ids are in the list AND whose
	// cardgroup is owned by ownerID. Returns the number of rows actually deleted
	// (cards owned by other users are silently skipped at SQL level so a single
	// foreign id in the list does not abort the batch).
	//
	// Empty ids short-circuits to (0, nil) without touching the DB. With an empty
	// slice GORM v2 omits the `WHERE id IN (?)` clause altogether, which would
	// convert this `Delete` into an unbounded mass delete — far worse than a slow scan.
	DeleteByIDs(ctx context.Context, ownerID string, ids []string) (int64, error)
	// UpsertManyTx upserts cards by (cardgroup_id, front), overwriting `back` and `position`;
	// the database trigger advances updated_at. Returns the per-row split between Inserted
	// and Updated. Empty input is a no-op. Inputs above bulkStatementChunkRows run as several
	// statements; tx must be a transaction so a later-chunk failure rolls back the earlier chunks.
	UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (UpsertManyTxResult, error)
	// FoldFrontCaseToTx renames one case-insensitive match per incoming front to
	// the incoming casing so a following UpsertManyTx updates it. Empty fronts
	// returns 0 without touching the database. Inputs above bulkStatementChunkRows
	// run as several statements; tx must be a transaction so a later-chunk failure
	// rolls back the earlier chunks. A concurrent insert of the exact front returns
	// ErrCardDuplicateFront.
	FoldFrontCaseToTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error)
}

type CardRepository interface {
	CardReadRepository
	CardPageRepository
	CardSessionRepository
	CardWriteRepository
}

type cardRepo struct{ db *gorm.DB }

func NewCardRepository(db *gorm.DB) CardRepository { return &cardRepo{db: db} }

func (r *cardRepo) FindByID(ctx context.Context, id string) (*domain.Card, error) {
	return findCardByID(ctx, r.db, id)
}

func (r *cardRepo) FindByIDForUpdateTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error) {
	return findCardByID(ctx, tx.Clauses(clause.Locking{Strength: "UPDATE"}), id)
}

func findCardByID(ctx context.Context, db *gorm.DB, id string) (*domain.Card, error) {
	var row gormCard
	err := db.WithContext(ctx).Where("id = ?", id).Take(&row).Error
	if err != nil {
		// Do not apply SQLSTATE 22P02 where another client-controlled bind could fail; id is the only one here.
		if errors.Is(err, gorm.ErrRecordNotFound) || pgInvalidTextRepresentation(err) {
			return nil, ErrNotFound
		}
		return nil, eris.Wrap(err, "repository: card: find by id")
	}
	return cardToDomain(row), nil
}

func (r *cardRepo) FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error) {
	if len(ids) == 0 {
		return map[string]*domain.Card{}, nil
	}
	var rows []gormCard
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: card: find by ids")
	}
	out := make(map[string]*domain.Card, len(rows))
	for i := range rows {
		card := cardToDomain(rows[i])
		out[card.ID] = card
	}
	return out, nil
}

func (r *cardRepo) Create(ctx context.Context, card *domain.Card) error {
	row := cardToRow(card)
	if err := r.db.WithContext(ctx).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "updated_at"}}}).
		Create(row).Error; err != nil {
		if classified := classifyCardDuplicateFront(err); classified != nil {
			return classified
		}
		if classified := classifyCardFKError(err); classified != nil {
			return classified
		}
		if classified := classifyTextLengthViolation(err); classified != nil {
			return classified
		}
		if classified := classifyFrontIndexRowTooLarge(err, cardsFrontIndex); classified != nil {
			return classified
		}
		return eris.Wrap(err, "repository: card: create")
	}
	card.UpdatedAt = row.UpdatedAt
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
		return nil, eris.Wrap(err, "repository: card: find by cardgroup and front")
	}
	return cardToDomain(row), nil
}

func (r *cardRepo) CountMatchingFrontsFold(ctx context.Context, cardgroupID string, loweredFronts []string) (int64, error) {
	if len(loweredFronts) == 0 {
		return 0, nil
	}
	var total int64
	// Not chunked on the raw input: a front repeated in two chunks would be counted twice.
	distinct := slices.Compact(slices.Sorted(slices.Values(loweredFronts)))
	for chunk := range slices.Chunk(distinct, bulkStatementChunkRows) {
		var count int64
		if err := r.db.WithContext(ctx).Raw(
			`SELECT COUNT(DISTINCT LOWER(front)) FROM cards
			 WHERE cardgroup_id = ? AND LOWER(front) IN ?`,
			cardgroupID, chunk,
		).Scan(&count).Error; err != nil {
			return 0, eris.Wrap(err, "repository: card: count matching fronts fold")
		}
		total += count
	}
	return total, nil
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
		if classified := classifyCardDuplicateFront(res.Error); classified != nil {
			return nil, classified
		}
		if classified := classifyTextLengthViolation(res.Error); classified != nil {
			return nil, classified
		}
		if classified := classifyFrontIndexRowTooLarge(res.Error, cardsFrontIndex); classified != nil {
			return nil, classified
		}
		return nil, eris.Wrap(res.Error, "repository: card: update")
	}
	return refetchAfterUpdate(res.RowsAffected, ErrNotFound,
		func() (*domain.Card, error) { return r.FindByID(ctx, id) }, "")
}

func (r *cardRepo) Delete(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&gormCard{})
	if res.Error != nil {
		return eris.Wrap(res.Error, "repository: card: delete")
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *cardRepo) DeleteByIDs(ctx context.Context, ownerID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// Owner check at SQL: cards.cardgroup_id must reference a cardgroup the
	// user owns. The subselect is the SOLE ownership gate — the usecase does
	// no read-side owner check, so foreign-owned ids in the list are silently
	// filtered out here. Do not remove the cardgroup_id IN (...) clause
	// without adding an equivalent guard upstream.
	res := r.db.WithContext(ctx).
		Where("id IN ? AND cardgroup_id IN (?)", ids,
			r.db.Model(&gormCardgroup{}).Select("id").Where("owner_id = ?", ownerID),
		).
		Delete(&gormCard{})
	if res.Error != nil {
		return 0, eris.Wrap(res.Error, "repository: bulk delete cards")
	}
	return res.RowsAffected, nil
}

func cardToRow(card *domain.Card) *gormCard {
	return &gormCard{
		ID:          card.ID,
		CardgroupID: string(card.CardgroupID),
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
		CardgroupID: domain.CardgroupID(row.CardgroupID),
		Front:       domain.CardText(row.Front),
		Back:        domain.CardText(row.Back),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
		Position:    row.Position,
	}
}
