package repository

import (
	"context"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
)

// gormMasterCatalogRow is the flat scan target used by findCatalogPage (and
// therefore by both FindPublishedPage and FindPageAnyStatus). It holds
// every master_cardgroups column as a flat field plus the derived card_count
// produced by the correlated COUNT. Embedding gormMasterCardgroup is
// intentionally avoided: gormMasterCardgroup carries a TableName() method that
// confuses GORM's embedded-struct schema parser when the outer scan target is a
// different type, silently leaving every embedded column at its Go zero value.
// See docs/backend/library-gotchas/gorm-embedded-tablename-scan-confusion.md.
type gormMasterCatalogRow struct {
	ID               string    `gorm:"column:id"`
	Name             string    `gorm:"column:name"`
	Description      *string   `gorm:"column:description"`
	Version          int       `gorm:"column:version"`
	Status           string    `gorm:"column:status"`
	IsDefaultStarter bool      `gorm:"column:is_default_starter"`
	SortOrder        int       `gorm:"column:sort_order"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
	CardCount        int64     `gorm:"column:card_count"`
}

// toGorm rebuilds the gormMasterCardgroup view of the row so the shared
// masterCardgroupToDomain conversion remains the single domain boundary.
func (r gormMasterCatalogRow) toGorm() gormMasterCardgroup {
	return gormMasterCardgroup{
		ID:               r.ID,
		Name:             r.Name,
		Description:      r.Description,
		Version:          r.Version,
		Status:           r.Status,
		IsDefaultStarter: r.IsDefaultStarter,
		SortOrder:        r.SortOrder,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

// CountCards returns the number of master cards belonging to the given master
// cardgroup.
func (r *masterCardgroupRepo) CountCards(ctx context.Context, masterCardgroupID string) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Table("master_cards").
		Where("master_cardgroup_id = ?", masterCardgroupID).
		Count(&total).Error; err != nil {
		return 0, eris.Wrap(err, "repository: master cardgroup: count cards")
	}
	return total, nil
}

// masterCatalogCursorSpec describes the catalog aggregate's cursor geometry.
// MasterCatalogOrderBy has no `id` member, so isIDOrder is always false and both
// the primary sort column and the `mcg.id` tie-break are always emitted. The
// columns are prefixed with the `mcg` alias used by findCatalogPage.
func masterCatalogCursorSpec(orderBy MasterCatalogOrderBy, c *MasterCatalogCursor) cursorSpec {
	return cursorSpec{
		alias:      "mcg",
		orderCol:   "mcg." + string(orderBy),
		isIDOrder:  false,
		fieldValue: func() (any, error) { return masterCatalogCursorFieldValue(orderBy, c) },
	}
}

// masterCatalogOrderClause renders the SQL ORDER BY tail with a secondary
// mcg.id tie-break so cursors stay deterministic when the primary sort column
// has duplicates. MasterCatalogOrderBy has no `id` member, so both columns are
// always emitted. The columns are prefixed with the `mcg` alias used by
// findCatalogPage.
func masterCatalogOrderClause(orderBy MasterCatalogOrderBy, dir SortOrder) string {
	return buildOrderClause(masterCatalogCursorSpec(orderBy, nil), dir)
}

// masterCatalogCursorWhere builds the tuple-comparison WHERE for the supplied
// cursor and direction. ASC yields `>`, DESC yields `<`. Returns an error when
// the cursor lacks the column required by the active orderBy — that is a caller
// bug, not user-supplied input. Columns are prefixed with the `mcg` alias used
// by findCatalogPage.
func masterCatalogCursorWhere(orderBy MasterCatalogOrderBy, dir SortOrder, c *MasterCatalogCursor) (string, []any, error) {
	return buildCursorWhere(masterCatalogCursorSpec(orderBy, c), dir, c.ID)
}

// masterCatalogCursorFieldValue returns the cursor value for the active orderBy
// field. The usecase layer hydrates the relevant column before calling
// FindPublishedPage or FindPageAnyStatus, so a missing column is a caller bug.
func masterCatalogCursorFieldValue(orderBy MasterCatalogOrderBy, c *MasterCatalogCursor) (any, error) {
	switch orderBy {
	case MasterCatalogOrderBySortOrder:
		if c.SortOrder != nil {
			return *c.SortOrder, nil
		}
	case MasterCatalogOrderByCreatedAt:
		if c.CreatedAt != nil {
			return *c.CreatedAt, nil
		}
	case MasterCatalogOrderByName:
		if c.Name != nil {
			return *c.Name, nil
		}
	}
	return nil, eris.Errorf("repository: master cardgroup: cursor missing %s column", orderBy)
}

// findCatalogPage is the shared cursor-paginated catalog engine. publishedOnly
// adds the public catalog-visibility filter — `status = published AND at least
// one master card exists` (see masterCardsExistPredicate) — and everything else
// is identical for the published and admin lists. FindPublishedPage and
// FindPageAnyStatus are thin wrappers over it
// (mirrors card_pagination.go's FindPageByCardgroup -> FindPageByCardgroupForUser).
func (r *masterCardgroupRepo) findCatalogPage(
	ctx context.Context,
	after, before *MasterCatalogCursor,
	first, last int,
	orderBy MasterCatalogOrderBy,
	dir SortOrder,
	search *string,
	publishedOnly bool,
) ([]*MasterCatalogItem, int64, error) {
	first = ClampPageSize(first)
	last = ClampPageSize(last)

	// totalCount comes from a COUNT(*) on the SAME filtered base (the
	// published + non-empty visibility filter when publishedOnly, plus the
	// optional search predicate), built
	// independently of the cursor/order/limit so it counts the whole matching
	// set. Computed before the no-rows short-circuit so callers asking only for
	// totalCount still observe the real count. Every predicate added here must
	// be added to the page query below and vice versa, or totalCount drifts
	// permanently away from the rows the caller can actually page through.
	countQ := r.db.WithContext(ctx).Model(&gormMasterCardgroup{})
	if publishedOnly {
		countQ = countQ.
			Where("status = ?", string(domain.MasterStatusPublished)).
			Where(masterCardsExistPredicate("master_cardgroups"))
	}
	if pattern, ok := searchLikePattern(search); ok {
		countQ = countQ.Where("name ILIKE ?", pattern)
	}
	var total int64
	if err := countQ.Count(&total).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: master cardgroup: count catalog page")
	}

	if first == 0 && last == 0 {
		return []*MasterCatalogItem{}, total, nil
	}

	// Backward paging executes the query with the inverted direction and
	// reverses the slice afterwards.
	effectiveDir, limit, cursor, reverse := paginateSetup(dir, first, last, after, before)

	q := r.db.WithContext(ctx).
		Table("master_cardgroups AS mcg").
		Select("mcg.*, (SELECT COUNT(*) FROM master_cards mc WHERE mc.master_cardgroup_id = mcg.id) AS card_count")
	if publishedOnly {
		q = q.
			Where("mcg.status = ?", string(domain.MasterStatusPublished)).
			Where(masterCardsExistPredicate("mcg"))
	}
	if pattern, ok := searchLikePattern(search); ok {
		q = q.Where("mcg.name ILIKE ?", pattern)
	}
	if cursor != nil {
		clauseSQL, args, err := masterCatalogCursorWhere(orderBy, effectiveDir, cursor)
		if err != nil {
			return nil, 0, eris.Wrap(err, "repository: master cardgroup: build catalog cursor where")
		}
		q = q.Where(clauseSQL, args...)
	}
	q = q.Order(masterCatalogOrderClause(orderBy, effectiveDir)).Limit(limit)

	var rows []gormMasterCatalogRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, 0, eris.Wrap(err, "repository: master cardgroup: find catalog page")
	}

	if reverse {
		ReverseSlice(rows)
	}

	out := make([]*MasterCatalogItem, len(rows))
	for i := range rows {
		cg, err := masterCardgroupToDomain(rows[i].toGorm())
		if err != nil {
			return nil, 0, err
		}
		out[i] = &MasterCatalogItem{
			Cardgroup: cg,
			CardCount: rows[i].CardCount,
		}
	}
	return out, total, nil
}

// FindPublishedPage returns the cursor-paginated published catalog list (drafts
// and published-but-empty decks excluded). Order is (orderBy, id) so cursors
// stay deterministic even when the primary sort column has duplicates.
func (r *masterCardgroupRepo) FindPublishedPage(
	ctx context.Context,
	after, before *MasterCatalogCursor,
	first, last int,
	orderBy MasterCatalogOrderBy,
	dir SortOrder,
	search *string,
) ([]*MasterCatalogItem, int64, error) {
	return r.findCatalogPage(ctx, after, before, first, last, orderBy, dir, search, true)
}

// FindPageAnyStatus returns the cursor-paginated admin catalog list (drafts and
// published-but-empty decks included). Identical to FindPublishedPage but
// without the catalog-visibility filter, so an admin can still find and fix a
// published deck that has lost all of its cards.
func (r *masterCardgroupRepo) FindPageAnyStatus(
	ctx context.Context,
	after, before *MasterCatalogCursor,
	first, last int,
	orderBy MasterCatalogOrderBy,
	dir SortOrder,
	search *string,
) ([]*MasterCatalogItem, int64, error) {
	return r.findCatalogPage(ctx, after, before, first, last, orderBy, dir, search, false)
}
