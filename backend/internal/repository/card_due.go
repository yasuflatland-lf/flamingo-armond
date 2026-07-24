package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/rotisserie/eris"
	"gorm.io/gorm"

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

// Rescue window:   due IS NOT NULL AND due < learn-day end AND last_review < learn-day start AND last_review <= now-24h.
// Filler window:   due IS NOT NULL AND due <= now AND last_review < learn-day start.
// Practice window: last_review >= boundary; due not consulted.
// Both usecase methods (NextDueCards and PracticeTodaysCards) derive the
// boundaries from the shared domain.StartOfLearnDay and domain.EndOfLearnDay
// formulas, computed once per call before hitting the repository. Changing the
// formula or comparator for the rescue, filler, or practice window without the
// others can make a card vanish from (or appear in) both queues.
// The `ucs.last_review < reviewedBefore` predicate is the exact complement of
// domain.ReviewedWithinLearnDay (the recording-side replay guard in
// usecase/swipe.go); a comparator edit on either side must move with the other.
//
// findDueCardsOn fetches the cards eligible for a learning session in three
// independent LIMIT windows and concatenates them: rescue reviews, filler
// reviews, then new (never-reviewed) cards. Splitting the fetch keeps a large
// filler or new-card backlog from evicting rescue reviews under a single LIMIT.
// The returned slice may hold up to 3*limit rows; OrderingPolicy (in the
// usecase) applies the final interleave and truncation per session.
//
// Rescue window: the card was last reviewed before window.ReviewedBefore (the
// caller's JST start-of-today), its latest rating was Again or its stability is
// below domain.LearnedStabilityDays, and its due is before window.RescueDueBefore
// (the exclusive JST end-of-today). The day-granular due bound deliberately
// surfaces rescue cards due later today. The early serve is floored by
// window.RescueReviewedBefore (domain.RescueReviewedBefore, 24 hours before now):
// FSRS counts elapsed days as floor(hours/24), so a repeat inside the same 24
// hours earns a stability growth factor of exactly zero and the rescue slot is
// wasted. random() varies selection within the band.
//
// Filler window: the same last-review guard excludes cards swiped today, but
// only non-rescue cards whose due has arrived (due <= window.Now) qualify. Its
// predicate is disjoint from the rescue window, so no review row is fetched
// twice. random() varies selection within the band.
//
// New window: no FSRS row yet; random() samples uniformly across the whole
// unseen pool so consecutive sessions surface different cards instead of
// walking the deterministic created_at/position (document) order.
func findDueCardsOn(db *gorm.DB, userID, cardgroupID string, window domain.LearnWindow, limit int) ([]domain.DueCard, error) {
	userID = coalesceUserIDForJoin(userID)
	if limit <= 0 {
		return []domain.DueCard{}, nil
	}

	// Only domain.RatingAgain and domain.LearnedStabilityDays are formatted into
	// the predicate: both are compile-time constants. Every caller-supplied
	// instant — including the window.RescueReviewedBefore floor — travels as a
	// bound query argument.
	rescueWhere := fmt.Sprintf(
		"cards.cardgroup_id = ? AND ucs.due IS NOT NULL AND ucs.due < ? AND ucs.last_review < ? AND ucs.last_review <= ? AND (ucs.last_rating = %d OR ucs.stability < %g)",
		domain.RatingAgain, domain.LearnedStabilityDays,
	)
	rescueRows, err := dueRowsOn(db, userID,
		rescueWhere,
		[]any{cardgroupID, window.RescueDueBefore, window.ReviewedBefore, window.RescueReviewedBefore},
		"random()",
		limit,
		"repository: card: find due cards")
	if err != nil {
		return nil, err
	}
	rescueCards, err := dueCardsFromRows(rescueRows)
	if err != nil {
		return nil, err
	}
	for i := range rescueCards {
		rescueCards[i].Rescue = true
	}

	fillerWhere := fmt.Sprintf(
		"cards.cardgroup_id = ? AND ucs.due IS NOT NULL AND ucs.due <= ? AND ucs.last_review < ? AND (ucs.last_rating IS DISTINCT FROM %d AND ucs.stability >= %g)",
		domain.RatingAgain, domain.LearnedStabilityDays,
	)
	fillerRows, err := dueRowsOn(db, userID,
		fillerWhere,
		[]any{cardgroupID, window.Now, window.ReviewedBefore},
		"random()",
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

	out := make([]domain.DueCard, 0, len(rescueCards)+len(fillerRows)+len(newRows))
	out = append(out, rescueCards...)
	for _, rows := range [][]dueCardRow{fillerRows, newRows} {
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
// WHERE predicate, ORDER BY clause, and LIMIT. Shared by all three fetches in
// findDueCardsOn and the practice fetch in findPracticeCardsOn so the
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
// defaulting Phase to FSRSPhaseNew and Due to created_at when the LEFT JOIN
// produced NULL FSRS columns (a new card). Rescue defaults to false; the caller
// marks rows from the rescue window after mapping.
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
