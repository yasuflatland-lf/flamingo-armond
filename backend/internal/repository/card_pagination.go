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

func (r *cardRepo) FindPageByCardgroupForUser(
	ctx context.Context,
	userID, cardgroupID string,
	after *CardCursor,
	first int,
	orderBy CardOrderBy,
	dir SortOrder,
	search *string,
) ([]*domain.Card, int64, map[string]time.Time, error) {
	userID = coalesceUserIDForJoin(userID)
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
		return nil, 0, nil, eris.Wrap(err, "repository: count cards by cardgroup")
	}

	if first == 0 {
		return []*domain.Card{}, total, nil, nil
	}

	q := base
	if orderBy == CardOrderByDue {
		// Project the ordering expression alongside the row so the caller can
		// mint a cursor from the SAME snapshot that ordered the page. The DUE
		// key lives on no card column, and recovering it afterwards would read a
		// different snapshot.
		q = q.Select("cards.*, "+cardCursorSpec(CardOrderByDue, nil).orderCol+" AS order_key").
			Joins("LEFT JOIN user_card_fsrs ucs ON ucs.user_id = ? AND ucs.card_id = cards.id", userID).
			Order(orderClause(orderBy, dir))
	} else {
		q = q.Order(orderClause(orderBy, dir))
	}

	if after != nil {
		clauseStr, args, err := cursorWhere(orderBy, dir, after)
		if err != nil {
			return nil, 0, nil, eris.Wrap(err, "repository: build cursor where")
		}
		q = q.Where(clauseStr, args...)
	}

	q = q.Limit(first)

	var rows []gormCard
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, nil, eris.Wrap(err, "repository: find page by cardgroup")
	}

	out := make([]*domain.Card, len(rows))
	for i := range rows {
		out[i] = cardToDomain(rows[i])
	}
	return out, total, cardPageOrderKeys(orderBy, rows), nil
}

// cardPageOrderKeys reports the value the page query ORDERED BY for each row it
// returned, keyed by card id. Every value comes out of the result set that
// produced the page, so a caller minting a v2 cursor embeds the boundary the
// page was actually served under — not a value re-read afterwards, which would
// come from a later snapshot and could disagree.
//
// The DUE key is read from the projected order_key alias because it is a
// COALESCE over a LEFT JOIN and lives on no card column. CREATED_AT and
// UPDATED_AT read their own column off the same row, which is the same snapshot
// by construction. ID returns nil: its ordering key IS the id, which the cursor
// already carries.
func cardPageOrderKeys(orderBy CardOrderBy, rows []gormCard) map[string]time.Time {
	if orderBy == CardOrderByID {
		return nil
	}
	keys := make(map[string]time.Time, len(rows))
	for _, row := range rows {
		switch orderBy {
		case CardOrderByDue:
			if row.OrderKey != nil {
				keys[row.ID] = *row.OrderKey
			}
		case CardOrderByCreatedAt:
			keys[row.ID] = row.CreatedAt
		case CardOrderByUpdatedAt:
			keys[row.ID] = row.UpdatedAt
		}
	}
	return keys
}

// cardCursorSpec describes the card aggregate's cursor geometry. The primary
// sort column is `cards.<orderBy>`, except CardOrderByDue which sorts on the
// COALESCE expression over the LEFT JOIN's nullable due column. The id tie-break
// is aliased `cards.id`.
func cardCursorSpec(orderBy CardOrderBy, c *CardCursor) cursorSpec {
	field := "cards." + string(orderBy)
	if orderBy == CardOrderByDue {
		field = "COALESCE(ucs.due, cards.created_at)"
	}
	return cursorSpec{
		alias:      "cards",
		orderCol:   field,
		isIDOrder:  orderBy == CardOrderByID,
		fieldValue: func() (any, error) { return cursorFieldValue(orderBy, c) },
	}
}

// orderClause renders the SQL ORDER BY tail. When orderBy is `id` only one
// column appears; otherwise the secondary `id` keeps order deterministic.
func orderClause(orderBy CardOrderBy, dir SortOrder) string {
	return buildOrderClause(cardCursorSpec(orderBy, nil), dir)
}

// cursorWhere builds the tuple-comparison WHERE for the supplied cursor and
// direction. ASC yields `>`, DESC yields `<`. Returns an error when the
// cursor lacks the column required by the active orderBy.
func cursorWhere(orderBy CardOrderBy, dir SortOrder, c *CardCursor) (string, []any, error) {
	return buildCursorWhere(cardCursorSpec(orderBy, c), dir, c.ID)
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
	return nil, eris.Errorf("repository: card: cursor missing %s column", orderBy)
}

func coalesceUserIDForJoin(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return "00000000-0000-0000-0000-000000000000"
	}
	return userID
}
