package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rotisserie/eris"
	"gorm.io/gorm"

	"backend/internal/domain"
)

// upsertParamsPerRow is the number of bind parameters one row contributes to upsertChunkTx's
// INSERT (id, <fkColumn>, front, back, created_at, position); it must equal upsertRowPlaceholders' arity.
const upsertParamsPerRow = 6

// upsertRowPlaceholders is one VALUES tuple of upsertChunkTx's INSERT.
const upsertRowPlaceholders = "(?, ?, ?, ?, ?, ?)"

// bulkStatementChunkRows caps rows (upsertManyTx) or fronts (FoldFrontCaseToTx, CountMatchingFrontsFold,
// deleteByGroupAndFrontsTx) per statement: pgx v5 pgconn/pgconn.go rejects more than 65,535 bind
// parameters before reaching the server ("extended protocol limited to 65535 parameters"), so one
// upsert statement holds at most 10,922 rows (65,535 / 6). 5,000 rows x 6 parameters = 30,000;
// a fronts chunk binds 5,001 (the fronts plus the group id).
const bulkStatementChunkRows = 5000

// UpsertManyTxResult counts the outcome of an UpsertManyTx call.
// Inserted+Updated equals len(input cards) for a successful call.
type UpsertManyTxResult struct {
	Inserted int64
	Updated  int64
}

// UpsertManyTx upserts cards by (cardgroup_id, front) on uq_cards_cardgroup_front, overwriting
// `back` and `position`; the trigger advances updated_at. RETURNING (xmax = 0) splits Inserted
// from Updated without a second query. It uses tx only, never r.db; inputs above
// bulkStatementChunkRows run as several statements, so tx must be a transaction for a failed
// chunk to roll back the earlier ones. Empty input returns a zero-valued result and no error.
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
		if classified := classifyCardFKError(err); classified != nil {
			return UpsertManyTxResult{}, classified
		}
		if classified := classifyTextLengthViolation(err); classified != nil {
			return UpsertManyTxResult{}, classified
		}
		if classified := classifyFrontIndexRowTooLarge(err, cardsFrontIndex); classified != nil {
			return UpsertManyTxResult{}, classified
		}
		return UpsertManyTxResult{}, eris.Wrap(err, "repository: card: upsert many")
	}
	return res, nil
}

// FoldFrontCaseToTx renames one case-insensitive match per front, choosing the oldest created_at then smallest id; extra matches stay
// untouched. LOWER-distinct input, NOT EXISTS, and DISTINCT ON prevent uq_cards_cardgroup_front conflicts with rows visible to the
// statement; an exact-front row a concurrent transaction commits after the statement's snapshot still collides, and that 23505 returns
// ErrCardDuplicateFront. Stable ids preserve FSRS/swipes; the DB-owned updated_at trigger fires [#1112]. Runs bulkStatementChunkRows
// fronts per statement on tx, which must be a transaction; LOWER-distinct input keeps chunks off each other's rows.
func (r *cardRepo) FoldFrontCaseToTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error) {
	if len(fronts) == 0 {
		return 0, nil
	}

	var total int64
	for chunk := range slices.Chunk(fronts, bulkStatementChunkRows) {
		n, err := foldFrontCaseChunkTx(ctx, tx, cardgroupID, chunk)
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func foldFrontCaseChunkTx(ctx context.Context, tx *gorm.DB, cardgroupID string, fronts []string) (int64, error) {
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
		if classified := classifyCardDuplicateFront(res.Error); classified != nil {
			return 0, classified
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

// upsertManyTx is the (fkColumn, front) upsert shared by cardRepo and masterCardRepo: one
// multi-row INSERT per bulkStatementChunkRows rows into tableName on tx, which must be a
// transaction so a failed chunk rolls back the earlier ones. A key repeated across two chunks
// counts as Updated in the later chunk; callers pass key-unique input (imports dedupe, master-deck
// copies inherit master_cards' unique front). Each table wrapper owns its "repository: <table>:" prefix.
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

	var res UpsertManyTxResult
	for chunk := range slices.Chunk(rows, bulkStatementChunkRows) {
		part, err := upsertChunkTx(ctx, tx, chunk, tableName, fkColumn)
		if err != nil {
			return UpsertManyTxResult{}, err
		}
		res.Inserted += part.Inserted
		res.Updated += part.Updated
	}
	return res, nil
}

func upsertChunkTx(ctx context.Context, tx *gorm.DB, rows []upsertCardRow, tableName, fkColumn string) (UpsertManyTxResult, error) {
	columns := "(id, " + fkColumn + ", front, back, created_at, position)"

	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(tableName)
	sb.WriteString(" ")
	sb.WriteString(columns)
	sb.WriteString(" VALUES ")
	args := make([]any, 0, len(rows)*upsertParamsPerRow)
	for i, row := range rows {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(upsertRowPlaceholders)
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
// the given group, scoped by fkColumn = groupID, ordered front ASC. Used by
// masterCardRepo; kept table-parameterized alongside upsertManyTx. The caller
// owns the layer-prefix wrap.
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

// deleteByGroupAndFrontsTx hard-deletes rows matching the scoped (fkColumn, front) natural key,
// one statement per bulkStatementChunkRows fronts on tx, which must be a transaction so a failed
// chunk rolls back the earlier ones. Used by masterCardRepo; the caller owns the
// layer-prefix wrap. Empty fronts returns (0, nil) without a query: GORM v2 drops an empty `IN ?`,
// deleting every row in the group (`.claude/rules/go-library-gotchas.md` § GORM empty IN).
func deleteByGroupAndFrontsTx(ctx context.Context, tx *gorm.DB, groupID string, fronts []string, tableName, fkColumn string) (int64, error) {
	if len(fronts) == 0 {
		return 0, nil
	}
	var total int64
	for chunk := range slices.Chunk(fronts, bulkStatementChunkRows) {
		res := tx.WithContext(ctx).
			Table(tableName).
			Where(fkColumn+" = ? AND front IN ?", groupID, chunk).
			Delete(nil)
		if res.Error != nil {
			return 0, eris.Wrap(res.Error, "delete by cardgroup and fronts")
		}
		total += res.RowsAffected
	}
	return total, nil
}
