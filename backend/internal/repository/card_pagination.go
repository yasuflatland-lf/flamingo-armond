package repository

import (
	"context"
	"strings"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
)

// SortOrder is the sort direction buildOrderClause and buildCursorWhere render.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// CardCursor is an ID boundary for paginated card queries.
type CardCursor struct {
	ID string
}

func (r *cardRepo) FindPageByCardgroup(
	ctx context.Context,
	cardgroupID string,
	after *CardCursor,
	first int,
	search *string,
) ([]*domain.Card, int64, error) {
	first = ClampPageSize(first)

	// Base query scoped to the cardgroup.
	base := r.db.WithContext(ctx).Model(&gormCard{}).Where("cards.cardgroup_id = ?", cardgroupID)

	// searchLikePattern trims, escapes LIKE metacharacters, and drops the
	// predicate when search is nil or blank, keeping blank-search handling
	// consistent across every paginated repository.
	if pattern, ok := searchLikePattern(search); ok {
		base = base.Where("(cards.front ILIKE ? OR cards.back ILIKE ?)", pattern, pattern)
	}

	// totalCount comes from a separate COUNT(*) scoped to the cardgroup (and
	// search filter, if active). Computed before the no-rows short-circuit so
	// callers passing first=0 still observe the real count.
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: count cards by cardgroup")
	}

	if first == 0 {
		return []*domain.Card{}, total, nil
	}

	q := base.Order(orderClause())

	if after != nil {
		clauseStr, args, err := cursorWhere(after)
		if err != nil {
			return nil, 0, eris.Wrap(err, "repository: build cursor where")
		}
		q = q.Where(clauseStr, args...)
	}

	q = q.Limit(first)

	var rows []gormCard
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: find page by cardgroup")
	}

	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, total, nil
}

func cardCursorSpec() cursorSpec {
	return cursorSpec{alias: "cards", orderCol: "cards.id", isIDOrder: true}
}

func orderClause() string {
	return buildOrderClause(cardCursorSpec(), SortAsc)
}

func cursorWhere(c *CardCursor) (string, []any, error) {
	return buildCursorWhere(cardCursorSpec(), SortAsc, c.ID)
}

func coalesceUserIDForJoin(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return "00000000-0000-0000-0000-000000000000"
	}
	return userID
}
