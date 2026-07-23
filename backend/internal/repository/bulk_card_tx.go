package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// UpsertManyTxResult counts the outcome of an UpsertManyTx call.
// Inserted+Updated equals len(input cards) for a successful call.
type UpsertManyTxResult struct {
	Inserted int64
	Updated  int64
}

// UpsertManyTx upserts cards by (cardgroup_id, front). Existing rows have
// `back` and `position` overwritten; the database trigger advances updated_at.
// The conflict key requires the unique index `uq_cards_cardgroup_front`
// (migration 20260430080000_initial_schema).
//
// Counts are derived per-row from the PostgreSQL system column `xmax`. A
// freshly inserted row has `xmax = 0` in the same transaction; a row updated
// via `ON CONFLICT DO UPDATE` has `xmax` set to the current transaction id.
// The RETURNING clause exposes `xmax = 0 AS inserted` so the split can be
// computed without a second query.
//
// The method is transaction-safe: it operates on the supplied tx only and
// never reaches back to r.db. Empty input returns a zero-valued result and
// no error.
func (r *cardRepo) UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (UpsertManyTxResult, error) {
	rows := make([]upsertCardRow, len(cards))
	for i, c := range cards {
		rows[i] = upsertCardRow{
			ID:        c.ID,
			GroupID:   string(c.CardgroupID),
			Front:     string(c.Front),
			Back:      string(c.Back),
			Position:  c.Position,
			CreatedAt: c.CreatedAt,
		}
	}
	res, err := upsertManyTx(ctx, tx, rows, "cards", "cardgroup_id")
	if err != nil {
		if classified := classifyTextLengthViolation(err); classified != nil {
			return UpsertManyTxResult{}, classified
		}
		return UpsertManyTxResult{}, eris.Wrap(err, "repository: card: upsert many")
	}
	return res, nil
}

// FoldFrontCaseToTx renames one case-insensitive match per front, choosing the
// oldest created_at then smallest id; extra matches stay untouched. LOWER-distinct
// input, NOT EXISTS, and DISTINCT ON prevent uq_cards_cardgroup_front conflicts.
// Stable ids preserve FSRS/swipes; the DB-owned updated_at trigger fires [#1112].
// Empty fronts returns without touching the database.
func (r *cardRepo) FoldFrontCaseToTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error) {
	if len(fronts) == 0 {
		return 0, nil
	}

	var sb strings.Builder
	sb.WriteString("WITH incoming(front) AS (VALUES ")
	args := make([]any, 0, len(fronts)+1)
	for i, front := range fronts {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?)")
		args = append(args, front)
	}
	sb.WriteString(`),
targets AS (
  SELECT DISTINCT ON (LOWER(c.front)) c.id, i.front AS new_front
  FROM cards c
  JOIN incoming i
    ON LOWER(c.front) = LOWER(i.front) AND c.front <> i.front
  WHERE c.cardgroup_id = ?
    AND NOT EXISTS (
      SELECT 1 FROM cards e
      WHERE e.cardgroup_id = c.cardgroup_id AND e.front = i.front
    )
  ORDER BY LOWER(c.front), c.created_at ASC, c.id ASC
)
UPDATE cards SET front = t.new_front FROM targets t WHERE cards.id = t.id`)
	args = append(args, cardgroupID)

	res := tx.WithContext(ctx).Exec(sb.String(), args...)
	if res.Error != nil {
		if errors.Is(res.Error, context.Canceled) || errors.Is(res.Error, context.DeadlineExceeded) {
			return 0, res.Error
		}
		return 0, eris.Wrap(res.Error, "repository: card: fold front case")
	}
	return res.RowsAffected, nil
}

func (r *cardRepo) DeleteByIDsTx(ctx context.Context, tx *gorm.DB, ownerID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// Owner check at SQL: cards.cardgroup_id must reference a cardgroup the
	// user owns. The subselect is the SOLE ownership gate — the usecase does
	// no read-side owner check, so foreign-owned ids in the list are silently
	// filtered out here. Do not remove the cardgroup_id IN (...) clause
	// without adding an equivalent guard upstream.
	res := tx.WithContext(ctx).
		Where("id IN ? AND cardgroup_id IN (?)", ids,
			tx.Model(&gormCardgroup{}).Select("id").Where("owner_id = ?", ownerID),
		).
		Delete(&gormCard{})
	if res.Error != nil {
		return 0, eris.Wrap(res.Error, "repository: bulk delete cards")
	}
	return res.RowsAffected, nil
}

func (r *cardRepo) DeleteByCardgroupAndFrontsTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error) {
	affected, err := deleteByGroupAndFrontsTx(ctx, tx, cardgroupID, fronts, "cards", "cardgroup_id")
	if err != nil {
		return 0, eris.Wrap(err, "repository: card: delete by cardgroup and fronts")
	}
	return affected, nil
}

// upsertCardRow is the domain-agnostic, normalized representation of a card row
// consumed by upsertManyTx. It mirrors exactly the columns that upsertManyTx
// writes — (id, <fkColumn>, front, back, created_at, position) — so
// any table sharing the (group_id, front) upsert shape (cards, master_cards)
// can reuse the helper without importing a domain type. GroupID maps to the
// foreign-key column named by upsertManyTx's fkColumn argument.
type upsertCardRow struct {
	ID        string
	GroupID   string
	Front     string
	Back      string
	Position  int
	CreatedAt time.Time
}

// upsertManyTx is the table-parameterized bulk upsert shared by cardRepo and the
// master_card repository. It builds a single multi-row INSERT into tableName and
// resolves conflicts on the (fkColumn, front) unique key, overwriting back and
// position. The updated_at trigger fires for either branch. The per-row
// insert/update split is derived from the
// PostgreSQL `xmax = 0` system-column trick in the RETURNING clause, avoiding a
// second query.
//
// rows is a normalized, domain-agnostic slice; callers map their domain type to
// upsertCardRow before invoking. Empty input returns a zero-valued result and no
// error. The conflict columns are derived from fkColumn because the only conflict
// target this helper supports is the (fkColumn, front) pair.
//
// This helper does NOT embed a caller-specific layer prefix in its wraps: each
// table's wrapper method owns its own "repository: <table>: ..." prefix.
func upsertManyTx(ctx context.Context, tx *gorm.DB, rows []upsertCardRow, tableName, fkColumn string) (UpsertManyTxResult, error) {
	if len(rows) == 0 {
		return UpsertManyTxResult{}, nil
	}

	// Pre-fill any missing IDs so the RETURNING clause classifies every row
	// the caller handed us. UUID v7 is the project-wide convention; v4
	// fallback is rejected per .claude/rules/go-library-gotchas.md.
	for i := range rows {
		if strings.TrimSpace(rows[i].ID) == "" {
			id, err := uuid.NewV7()
			if err != nil {
				return UpsertManyTxResult{}, eris.Wrap(err, "uuid v7")
			}
			rows[i].ID = id.String()
		}
	}

	// Build a single multi-row INSERT. Each row contributes 6 placeholders
	// matching the column list below.
	columns := "(id, " + fkColumn + ", front, back, created_at, position)"
	const rowPH = "(?, ?, ?, ?, ?, ?)"

	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(tableName)
	sb.WriteString(" ")
	sb.WriteString(columns)
	sb.WriteString(" VALUES ")
	args := make([]any, 0, len(rows)*6)
	for i, row := range rows {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(rowPH)
		args = append(args,
			row.ID,
			row.GroupID,
			row.Front,
			row.Back,
			row.CreatedAt,
			row.Position,
		)
	}
	sb.WriteString("\n        ON CONFLICT (")
	sb.WriteString(fkColumn)
	sb.WriteString(", front)\n        DO UPDATE SET back = EXCLUDED.back, position = EXCLUDED.position\n        RETURNING (xmax = 0) AS inserted")

	type returnedRow struct {
		Inserted bool `gorm:"column:inserted"`
	}
	var returned []returnedRow
	if err := tx.WithContext(ctx).Raw(sb.String(), args...).Scan(&returned).Error; err != nil {
		return UpsertManyTxResult{}, eris.Wrap(err, "upsert many")
	}

	if len(returned) != len(rows) {
		return UpsertManyTxResult{}, eris.Errorf(
			"upsert many: returned %d rows, expected %d",
			len(returned), len(rows),
		)
	}

	var res UpsertManyTxResult
	for _, r := range returned {
		if r.Inserted {
			res.Inserted++
		} else {
			res.Updated++
		}
	}
	return res, nil
}

// listFrontsByGroupTx returns the sorted distinct-by-row `front` values for
// the given group, scoped by fkColumn = groupID, ordered front ASC. Shared by
// cardRepo and the master_card repository. The caller owns the layer-prefix wrap.
func listFrontsByGroupTx(ctx context.Context, tx *gorm.DB, groupID, tableName, fkColumn string) ([]string, error) {
	var fronts []string
	if err := tx.WithContext(ctx).
		Table(tableName).
		Where(fkColumn+" = ?", groupID).
		Order("front ASC").
		Pluck("front", &fronts).Error; err != nil {
		return nil, eris.Wrap(err, "list fronts by cardgroup")
	}
	return fronts, nil
}

// deleteByGroupAndFrontsTx hard-deletes rows matching the scoped
// (fkColumn, front) natural key. Shared by cardRepo and the master_card
// repository; the caller owns the layer-prefix wrap.
//
// Empty fronts short-circuits to (0, nil) without touching the DB. With an empty
// slice GORM v2 omits the `WHERE front IN (?)` clause altogether, which would
// convert this `Delete` into a delete-all-rows-in-group. See
// `.claude/rules/go-library-gotchas.md` § GORM empty IN.
func deleteByGroupAndFrontsTx(ctx context.Context, tx *gorm.DB, groupID string, fronts []string, tableName, fkColumn string) (int64, error) {
	if len(fronts) == 0 {
		return 0, nil
	}
	res := tx.WithContext(ctx).
		Table(tableName).
		Where(fkColumn+" = ? AND front IN ?", groupID, fronts).
		Delete(nil)
	if res.Error != nil {
		return 0, eris.Wrap(res.Error, "delete by cardgroup and fronts")
	}
	return res.RowsAffected, nil
}
