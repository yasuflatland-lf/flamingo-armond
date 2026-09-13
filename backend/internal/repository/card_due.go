package repository

import (
	"context"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"backend/internal/domain"
)

func (r *cardRepo) FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, window domain.LearnWindow, limit int) ([]domain.DueCard, error) {
	return findDueCardsOn(r.db.WithContext(ctx), userID, cardgroupID, window, limit)
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

// findDueCardsOn fetches review (due IS NOT NULL, due < DueBefore, last_review < ReviewedBefore and < CreditReviewedBefore) and unseen windows.
// The guards enforce JST learn-day exclusion and UTC-date scheduling credit; due uses the exclusive JST day end.
// Reviews sort by floor(elapsed days)/stability ASC: parameters.go:ForgettingCurve and arithmetic.go:decayAndFactor
// make R decrease with t/S; steps.go:dateDiffRaw floors elapsed days, and arithmetic.go:constrainStability clamps stability.
// Up to 2*limit rows return, reviews first then newest-added cards; OrderingPolicy interleaves without reordering either kind.
func findDueCardsOn(db *gorm.DB, userID, cardgroupID string, window domain.LearnWindow, limit int) ([]domain.DueCard, error) {
	userID = coalesceUserIDForJoin(userID)
	if limit <= 0 {
		return []domain.DueCard{}, nil
	}

	reviewRows, err := dueRowsOn(db, userID,
		"cards.cardgroup_id = ? AND ucs.due IS NOT NULL AND ucs.due < ? AND ucs.last_review < ? AND ucs.last_review < ?",
		[]any{cardgroupID, window.DueBefore, window.ReviewedBefore, window.CreditReviewedBefore},
		clause.Expr{
			SQL:  "floor(extract(epoch FROM (?::timestamptz - ucs.last_review)) / 86400) / greatest(ucs.stability, 0.001) ASC, ucs.due ASC, cards.id ASC",
			Vars: []any{window.Now},
		},
		limit,
		"repository: card: find due cards")
	if err != nil {
		return nil, err
	}

	newRows, err := dueRowsOn(db, userID,
		"cards.cardgroup_id = ? AND ucs.due IS NULL",
		[]any{cardgroupID},
		clause.Expr{SQL: "cards.created_at DESC, cards.position DESC, cards.id DESC"},
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
		clause.Expr{SQL: "random()"},
		limit,
		"repository: card: find practice cards")
	if err != nil {
		return nil, err
	}
	return dueCardsFromRows(rows)
}

// dueRowsOn runs the cards-with-FSRS LEFT JOIN scoped to userID with the given
// WHERE predicate, ORDER BY clause, and LIMIT. Shared by both fetches in
// findDueCardsOn and the practice fetch in findPracticeCardsOn so the
// SELECT/JOIN never drift between them. wrapMsg is supplied by the caller because
// a shared helper must not embed a caller-specific layer prefix.
func dueRowsOn(db *gorm.DB, userID, where string, whereArgs []any, order clause.Expression, limit int, wrapMsg string) ([]dueCardRow, error) {
	var rows []dueCardRow
	// ucs.last_review is used in WHERE clauses by both callers (findDueCardsOn
	// review window and findPracticeCardsOn) but is not projected into
	// dueCardRow — it is filter-only and not needed after scan.
	if err := db.
		Table("cards").
		Select("cards.id, cards.cardgroup_id, cards.front, cards.back, cards.created_at, cards.updated_at, cards.position, ucs.state, ucs.due").
		Joins("LEFT JOIN user_card_fsrs ucs ON ucs.user_id = ? AND ucs.card_id = cards.id", userID).
		Where(where, whereArgs...).
		Order(clause.OrderBy{Expression: order}).
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, wrapMsg)
	}
	return rows, nil
}

// dueCardsFromRows maps raw dueCardRow scan results into domain.DueCard values,
// defaulting Phase to FSRSPhaseNew and Due to created_at when the LEFT JOIN
// produced NULL FSRS columns (a new card).
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
		dc := domain.DueCard{Card: c, Phase: domain.FSRSPhaseNew, Due: r.CreatedAt}
		if r.State != nil {
			s := domain.FSRSPhase(*r.State)
			if !s.IsValid() {
				return nil, eris.Errorf("repository: card: invalid FSRSPhase %d", *r.State)
			}
			dc.Phase = s
		}
		if r.Due != nil {
			dc.Due = *r.Due
		}
		out[i] = dc
	}
	return out, nil
}
