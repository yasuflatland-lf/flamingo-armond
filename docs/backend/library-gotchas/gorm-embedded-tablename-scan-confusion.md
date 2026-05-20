# GORM embedded struct with `TableName()` silently breaks the outer scan

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A scan target that embeds another row struct inherits the embedded type's
`TableName()` method via Go's method promotion. GORM's schema parser then
resolves the embedded type's schema (its columns, its `TableName`) instead of
the outer struct's, and column-to-field mapping silently breaks. The outer
struct's own non-overlapping fields (extra columns from a JOIN, for example)
either never populate or scan into the embedded fields by accident.

The failure mode is silent: no scan error, no log line. Affected fields land
with the Go zero value. For string primary keys this means `id == ""` for
every row — easy to miss in tests that only inspect the joined columns.

```go
// Anti-pattern: dueCardRow embeds gormCard, which carries TableName()
type dueCardRow struct {
    gormCard            // TableName() returns "cards"
    State *int       `gorm:"column:state"` // from LEFT JOIN
    Due   *time.Time `gorm:"column:due"`
}

// rows[i].ID is empty string after Find; the outer struct's id mapping
// got displaced by the embedded schema lookup.
```

The fix is to **flatten the scan target** so no embedded type carries a
`TableName()`, and call `Table("cards").Select(...)` to set the table and
column projection explicitly:

```go
// Correct: flat struct, no embedded TableName() method
type dueCardRow struct {
    ID          string     `gorm:"column:id"`
    CardgroupID string     `gorm:"column:cardgroup_id"`
    Front       string     `gorm:"column:front"`
    Back        string     `gorm:"column:back"`
    CreatedAt   time.Time  `gorm:"column:created_at"`
    UpdatedAt   time.Time  `gorm:"column:updated_at"`
    State       *int       `gorm:"column:state"`
    Due         *time.Time `gorm:"column:due"`
}

err := db.
    Table("cards").
    Select("cards.id, cards.cardgroup_id, cards.front, cards.back, cards.created_at, cards.updated_at, ucs.state, ucs.due").
    Joins("LEFT JOIN user_card_fsrs ucs ON ucs.user_id = ? AND ucs.card_id = cards.id", userID).
    Where("cards.cardgroup_id = ? AND (ucs.due IS NULL OR ucs.due <= ?)", cardgroupID, now).
    Order("COALESCE(ucs.due, cards.created_at) ASC, cards.id ASC").
    Limit(limit).
    Find(&rows).Error
```

A struct comment at the declaration site documents the constraint so a future
reader does not re-embed `gormCard` for convenience:

```go
// dueCardRow is the raw scan target for findDueCardsOn. It holds all cards.*
// columns as flat fields plus nullable FSRS columns from the LEFT JOIN.
// Embedding gormCard is intentionally avoided: gormCard carries a TableName()
// method that confuses GORM's embedded-struct schema parser when the outer
// scan target is a different type.
```

**Why not `db.Model(&gormCard{}).Select(...).Find(&rows)`:** `Model` sets the
table but leaves the embedded-schema confusion intact when `rows` itself
embeds a typed row. The flat scan target + explicit `Table` is the smallest
fix that survives future column additions.

**Reference:** `backend/internal/repository/card.go` — `dueCardRow` (flat) and
`findDueCardsOn`. The same flat-scan pattern applies to any LEFT JOIN whose
right side contributes nullable columns to the projection.
