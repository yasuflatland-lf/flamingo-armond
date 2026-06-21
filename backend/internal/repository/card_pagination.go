package repository

import (
	"context"
	"strings"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
)

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

	if first == 0 && last == 0 {
		return []*domain.Card{}, total, nil
	}

	// Backward paging executes the query with the inverted direction and
	// reverses the slice afterwards.
	effectiveDir, limit, cursor, reverse := paginateSetup(dir, first, last, after, before)

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
	clause, args := cursorTupleWhere("cards", field, op, val, c.ID)
	return clause, args, nil
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

func coalesceUserIDForJoin(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return "00000000-0000-0000-0000-000000000000"
	}
	return userID
}
