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

// ErrCardDuplicateFront is returned by Create when an INSERT collides with the
// (cardgroup_id, front) unique index. Standalone — do NOT join with ErrNotFound;
// the row was found, which is precisely the failure (see
// docs/backend/error-wrapping/standalone-sentinels-not-every-joins-errnotfound.md).
var ErrCardDuplicateFront = errors.New("repository: card with same front exists in cardgroup")

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
	FindByIDForUpdateTx(ctx context.Context, tx *gorm.DB, id string) (*domain.Card, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]*domain.Card, error)
	ListByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error)
	ListFrontsByCardgroupTx(ctx context.Context, tx *gorm.DB, cardgroupID string) ([]string, error)
	// FindByCardgroupAndFront returns the card identified by the (cardgroup_id,
	// front) unique key, or ErrNotFound when no such row exists. The front value
	// is matched exactly; trimming is the caller's responsibility.
	FindByCardgroupAndFront(ctx context.Context, cardgroupID, front string) (*domain.Card, error)
	// CountExistingFronts returns how many of fronts already exist in the
	// destination cardgroup's cards, matched case-sensitively (plain text
	// equality, mirroring the uq_cards_cardgroup_front unique index the merge
	// upserts against). Empty fronts returns 0 without a query.
	CountExistingFronts(ctx context.Context, cardgroupID string, fronts []string) (int64, error)
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
}

// CardSessionRepository reads a learn/practice session's card pool. Unlike
// CardPageRepository these are non-paginated, limit-capped session fetches:
// no cursor, no orderBy, no totalCount.
type CardSessionRepository interface {
	// FindDueCardsForUser returns rescue reviews due before rescueDueBefore,
	// filler reviews due by now, and never-seen cards in independent windows.
	FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, now, reviewedBefore, rescueDueBefore time.Time, limit int) ([]domain.DueCard, error)
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
	// before invoking this method.
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
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

func (r *cardRepo) ListByCardgroup(ctx context.Context, cardgroupID string) ([]*domain.Card, error) {
	var rows []gormCard
	if err := r.db.WithContext(ctx).
		Where("cardgroup_id = ?", cardgroupID).
		Order("created_at ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: card: find by cardgroup")
	}
	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, nil
}

func (r *cardRepo) ListFrontsByCardgroupTx(ctx context.Context, tx *gorm.DB, cardgroupID string) ([]string, error) {
	fronts, err := listFrontsByGroupTx(ctx, tx, cardgroupID, "cards", "cardgroup_id")
	if err != nil {
		return nil, eris.Wrap(err, "repository: card: list fronts by cardgroup")
	}
	return fronts, nil
}

func (r *cardRepo) Create(ctx context.Context, card *domain.Card) error {
	if err := r.db.WithContext(ctx).Create(cardToRow(card)).Error; err != nil {
		if pgConstraintViolation(err, "23505", "uq_cards_cardgroup_front") {
			return ErrCardDuplicateFront
		}
		return eris.Wrap(err, "repository: card: create")
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
		return nil, eris.Wrap(err, "repository: card: find by cardgroup and front")
	}
	return cardToDomain(row), nil
}

func (r *cardRepo) CountExistingFronts(ctx context.Context, cardgroupID string, fronts []string) (int64, error) {
	if len(fronts) == 0 {
		return 0, nil
	}
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&gormCard{}).
		Where("cardgroup_id = ? AND front IN ?", cardgroupID, fronts).
		Count(&count).Error; err != nil {
		return 0, eris.Wrap(err, "repository: card: count existing fronts")
	}
	return count, nil
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
