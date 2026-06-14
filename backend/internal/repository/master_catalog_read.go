package repository

import (
	"context"
	"strings"
	"time"

	"github.com/rotisserie/eris"

	"backend/internal/domain"
)

// gormMasterCatalogRow is the flat scan target used by findCatalogPage (and
// therefore by both FindPublishedPage and FindAdminPage). It holds
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
	Language         *string   `gorm:"column:language"`
	Level            *string   `gorm:"column:level"`
	Category         *string   `gorm:"column:category"`
	CoverImageURL    *string   `gorm:"column:cover_image_url"`
	Source           *string   `gorm:"column:source"`
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
		Language:         r.Language,
		Level:            r.Level,
		Category:         r.Category,
		CoverImageURL:    r.CoverImageURL,
		Source:           r.Source,
		Version:          r.Version,
		Status:           r.Status,
		IsDefaultStarter: r.IsDefaultStarter,
		SortOrder:        r.SortOrder,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

// CountPublished returns the total number of published master cardgroups
// matching the optional search predicate. The published filter is enforced in
// SQL and is never caller-overridable.
func (r *masterCardgroupRepo) CountPublished(ctx context.Context, search *string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&gormMasterCardgroup{}).
		Where("status = ?", string(domain.MasterStatusPublished))
	if pattern, ok := masterCatalogSearchPattern(search); ok {
		q = q.Where("name ILIKE ?", pattern)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, eris.Wrap(err, "repository: master cardgroup: count published")
	}
	return total, nil
}

// CountAdmin returns the total number of master cardgroups of any status
// (draft or published) matching the optional search predicate.
func (r *masterCardgroupRepo) CountAdmin(ctx context.Context, search *string) (int64, error) {
	q := r.db.WithContext(ctx).Model(&gormMasterCardgroup{})
	if pattern, ok := masterCatalogSearchPattern(search); ok {
		q = q.Where("name ILIKE ?", pattern)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return 0, eris.Wrap(err, "repository: master cardgroup: count admin")
	}
	return total, nil
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

// masterCatalogSearchPattern wraps a non-empty trimmed search term in `%...%`
// after escaping LIKE metacharacters. Returns (pattern, false) when the search
// is nil or trims empty so callers can skip the predicate.
func masterCatalogSearchPattern(search *string) (string, bool) {
	if search == nil {
		return "", false
	}
	trimmed := strings.TrimSpace(*search)
	if trimmed == "" {
		return "", false
	}
	return "%" + escapeLikePattern(trimmed) + "%", true
}

// masterCatalogOrderClause renders the SQL ORDER BY tail with a secondary
// mcg.id tie-break so cursors stay deterministic when the primary sort column
// has duplicates. MasterCatalogOrderBy has no `id` member, so both columns are
// always emitted. The columns are prefixed with the `mcg` alias used by
// findCatalogPage.
func masterCatalogOrderClause(orderBy MasterCatalogOrderBy, dir SortOrder) string {
	return "mcg." + string(orderBy) + " " + string(dir) + ", mcg.id " + string(dir)
}

// masterCatalogCursorWhere builds the tuple-comparison WHERE for the supplied
// cursor and direction. ASC yields `>`, DESC yields `<`. Returns an error when
// the cursor lacks the column required by the active orderBy — that is a caller
// bug, not user-supplied input. Columns are prefixed with the `mcg` alias used
// by findCatalogPage.
func masterCatalogCursorWhere(orderBy MasterCatalogOrderBy, dir SortOrder, c *MasterCatalogCursor) (string, []any, error) {
	op := ">"
	if dir == SortDesc {
		op = "<"
	}
	field := "mcg." + string(orderBy)
	val, err := masterCatalogCursorFieldValue(orderBy, c)
	if err != nil {
		return "", nil, err
	}
	// Tuple compare: (field, id) op (val, c.ID). Expanded form is portable
	// across SQL dialects (Postgres-only `(a, b) > (?, ?)` avoided).
	return "(" + field + " " + op + " ? OR (" + field + " = ? AND mcg.id " + op + " ?))",
		[]any{val, val, c.ID}, nil
}

// masterCatalogCursorFieldValue returns the cursor value for the active orderBy
// field. The usecase layer hydrates the relevant column before calling
// FindPublishedPage or FindAdminPage, so a missing column is a caller bug.
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
	return nil, eris.Errorf("repository: master cardgroup: catalog cursor missing %s column", orderBy)
}

// findCatalogPage is the shared cursor-paginated catalog engine. publishedOnly
// adds the status filter; everything else is identical for the published and
// admin lists. FindPublishedPage and FindAdminPage are thin wrappers over it
// (mirrors card_pagination.go's FindPageByCardgroup -> FindPageByCardgroupForUser).
func (r *masterCardgroupRepo) findCatalogPage(
	ctx context.Context,
	after, before *MasterCatalogCursor,
	first, last int,
	orderBy MasterCatalogOrderBy,
	dir SortOrder,
	search *string,
	publishedOnly bool,
) ([]*MasterCatalogItem, error) {
	first = ClampPageSize(first)
	last = ClampPageSize(last)

	if first == 0 && last == 0 {
		return []*MasterCatalogItem{}, nil
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

	q := r.db.WithContext(ctx).
		Table("master_cardgroups AS mcg").
		Select("mcg.*, (SELECT COUNT(*) FROM master_cards mc WHERE mc.master_cardgroup_id = mcg.id) AS card_count")
	if publishedOnly {
		q = q.Where("mcg.status = ?", string(domain.MasterStatusPublished))
	}
	if pattern, ok := masterCatalogSearchPattern(search); ok {
		q = q.Where("mcg.name ILIKE ?", pattern)
	}
	if cursor != nil {
		clauseSQL, args, err := masterCatalogCursorWhere(orderBy, effectiveDir, cursor)
		if err != nil {
			return nil, eris.Wrap(err, "repository: master cardgroup: build catalog cursor where")
		}
		q = q.Where(clauseSQL, args...)
	}
	q = q.Order(masterCatalogOrderClause(orderBy, effectiveDir)).Limit(limit)

	var rows []gormMasterCatalogRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, eris.Wrap(err, "repository: master cardgroup: find catalog page")
	}

	if reverse {
		ReverseSlice(rows)
	}

	out := make([]*MasterCatalogItem, len(rows))
	for i := range rows {
		out[i] = &MasterCatalogItem{
			Cardgroup: masterCardgroupToDomain(rows[i].toGorm()),
			CardCount: rows[i].CardCount,
		}
	}
	return out, nil
}

// FindPublishedPage returns the cursor-paginated published catalog list (drafts
// excluded). Order is (orderBy, id) so cursors stay deterministic even when the
// primary sort column has duplicates.
func (r *masterCardgroupRepo) FindPublishedPage(
	ctx context.Context,
	after, before *MasterCatalogCursor,
	first, last int,
	orderBy MasterCatalogOrderBy,
	dir SortOrder,
	search *string,
) ([]*MasterCatalogItem, error) {
	return r.findCatalogPage(ctx, after, before, first, last, orderBy, dir, search, true)
}

// FindAdminPage returns the cursor-paginated admin catalog list (drafts
// included). Identical to FindPublishedPage but without the status filter.
func (r *masterCardgroupRepo) FindAdminPage(
	ctx context.Context,
	after, before *MasterCatalogCursor,
	first, last int,
	orderBy MasterCatalogOrderBy,
	dir SortOrder,
	search *string,
) ([]*MasterCatalogItem, error) {
	return r.findCatalogPage(ctx, after, before, first, last, orderBy, dir, search, false)
}
