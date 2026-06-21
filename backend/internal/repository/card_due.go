package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

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
// boundary from the same domain.StartOfLearnDay formula, computed once per call
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
// already reviewed at or after the boundary (the same domain.StartOfLearnDay cutoff the
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
			CardgroupID: domain.CardgroupID(r.CardgroupID),
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
