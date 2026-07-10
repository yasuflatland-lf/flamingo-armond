# Table-parameterized bulk repository helper

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When two aggregates share identical bulk SQL semantics — an ON CONFLICT upsert,
a list-fronts query, and a delete-by-fronts — extract package-private helpers
parameterized by `(tableName, fkColumn string)`. Each aggregate's repository
passes its own table name and FK column; neither imports the other's domain type.

## Why

`cards` and `master_cards` share the same `(group_id, front)` upsert shape:

| Table | FK column | Unique conflict key |
|---|---|---|
| `cards` | `cardgroup_id` | `(cardgroup_id, front)` |
| `master_cards` | `master_cardgroup_id` | `(master_cardgroup_id, front)` |

Without shared helpers, each repo duplicates ~270 lines of SQL construction,
`xmax = 0` insert-vs-update counting, and the fragile GORM empty-slice guard.
Duplicated helpers drift independently — a bug fix or a column addition applied
to one copy silently misses the other.

## The shared normalized row struct

```go
// backend/internal/repository/card.go

// upsertCardRow is the domain-agnostic, normalized representation of a card row
// consumed by upsertManyTx. GroupID maps to the FK column named by
// upsertManyTx's fkColumn argument.
type upsertCardRow struct {
    ID        string
    GroupID   string
    Front     string
    Back      string
    Position  int
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

The struct has no domain imports. `GroupID` maps to `fkColumn` at SQL-build
time; each aggregate's repo fills it from its own domain field.

## The three shared helpers

All three live in `backend/internal/repository/card.go` (package-private, not
exported):

### `upsertManyTx`

```go
func upsertManyTx(
    ctx context.Context,
    tx *gorm.DB,
    rows []upsertCardRow,
    tableName, fkColumn string,
) (UpsertManyTxResult, error)
```

Builds a single multi-row `INSERT INTO tableName ... ON CONFLICT (fkColumn, front) DO UPDATE`.
The per-row insert-vs-update split uses PostgreSQL's `RETURNING (xmax = 0) AS inserted`
system-column trick — no second query needed. Empty input returns a zero-valued result
with no error. Pre-fills blank IDs via `uuid.NewV7()` (no v4 fallback — see
[`go-library-gotchas.md` § "`uuid.NewV7` failure must propagate"](../../../.claude/rules/go-library-gotchas.md)).

### `listFrontsByGroupTx`

```go
func listFrontsByGroupTx(
    ctx context.Context,
    tx *gorm.DB,
    groupID, tableName, fkColumn string,
) ([]string, error)
```

Returns sorted `front` values for the group via `WHERE fkColumn = ? ORDER BY front ASC`.

### `deleteByGroupAndFrontsTx`

```go
func deleteByGroupAndFrontsTx(
    ctx context.Context,
    tx *gorm.DB,
    groupID string,
    fronts []string,
    tableName, fkColumn string,
) (int64, error)
```

Hard-deletes rows by the scoped `(fkColumn, front)` natural key. **Empty `fronts`
short-circuits to `(0, nil)` before touching the DB.** Without this guard, GORM
silently drops `WHERE front IN (?)` for an empty slice and deletes every row in
the group — the same trap described in
[`go-library-gotchas.md` § "GORM `WHERE id IN ?` with an empty slice returns all rows"](../../../.claude/rules/go-library-gotchas.md#gorm-where-id-in--with-an-empty-slice-returns-all-rows).
Every caller inherits the guard for free.

## How each aggregate wires the helpers

### `cardRepo` (table `"cards"`, FK `"cardgroup_id"`)

```go
func (r *cardRepo) UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.Card) (UpsertManyTxResult, error) {
    rows := make([]upsertCardRow, len(cards))
    for i, c := range cards {
        rows[i] = upsertCardRow{
            ID: c.ID, GroupID: c.CardgroupID,
            Front: string(c.Front), Back: string(c.Back),
            Position: c.Position, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
        }
    }
    res, err := upsertManyTx(ctx, tx, rows, "cards", "cardgroup_id")
    if err != nil {
        return UpsertManyTxResult{}, eris.Wrap(err, "repository: card: upsert many")
    }
    return res, nil
}
```

### `masterCardRepo` (table `"master_cards"`, FK `"master_cardgroup_id"`)

```go
func (r *masterCardRepo) UpsertManyTx(ctx context.Context, tx *gorm.DB, cards []*domain.MasterCard) (UpsertManyTxResult, error) {
    rows := make([]upsertCardRow, len(cards))
    for i, c := range cards {
        rows[i] = upsertCardRow{
            ID: c.ID, GroupID: c.MasterCardgroupID,
            Front: string(c.Front), Back: string(c.Back),
            Position: c.Position, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
        }
    }
    res, err := upsertManyTx(ctx, tx, rows, "master_cards", "master_cardgroup_id")
    if err != nil {
        return UpsertManyTxResult{}, eris.Wrap(err, "repository: master card: upsert many")
    }
    return res, nil
}
```

The domain-to-row mapping is the only aggregate-specific code. The helper body
is shared.

## Caller owns the eris layer prefix

The shared helper bodies carry **no fixed `layer:` prefix** in their `eris.Wrap`
calls (`"upsert many"`, `"list fronts by cardgroup"`, `"delete by cardgroup and fronts"`).
Each wrapper supplies its own aggregate-scoped prefix (`"repository: card: ..."` vs.
`"repository: master card: ..."`) so the logged `error_chain` attribute is attributed
to the right aggregate. Embedding a fixed prefix in the helper body would displace the
caller-specific module attribution.

This is the same discipline described in
[`.claude/rules/error-wrapping.md` § "Shared helpers must not embed a layer prefix wrap"](../../../.claude/rules/error-wrapping.md)
and demonstrated in `dueRowsOn` (see
[`repo-tx-and-nontx-share-private-helper.md`](repo-tx-and-nontx-share-private-helper.md)).

## Extending to a third aggregate

To reuse the helpers for a new table (e.g. `practice_cards / practice_cardgroup_id`):

1. Map `[]*domain.PracticeCard` to `[]upsertCardRow`, filling `GroupID` from
   `c.PracticeCardgroupID`.
2. Call the three helpers with the new `tableName` and `fkColumn` strings.
3. Wrap each helper's error with the new aggregate's prefix.

No changes to the shared helpers or to `card.go` / `master_card.go`.

## Reference

- `backend/internal/repository/card.go` — `upsertCardRow`, `upsertManyTx`,
  `listFrontsByGroupTx`, `deleteByGroupAndFrontsTx`.
- `backend/internal/repository/master_card.go` — `masterCardRepo.UpsertManyTx`,
  `ListFrontsByMasterCardgroupTx`, `DeleteByMasterCardgroupAndFrontsTx`.
- [`repo-tx-and-nontx-share-private-helper.md`](repo-tx-and-nontx-share-private-helper.md)
  — the sibling Tx/non-Tx drift-avoidance pattern and the `wrapMsg` parameter convention.
