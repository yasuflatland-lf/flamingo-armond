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
    Position    int        `gorm:"column:position"`
    State       *int       `gorm:"column:state"`
    Due         *time.Time `gorm:"column:due"`
}

err := db.
    Table("cards").
    Select("cards.id, cards.cardgroup_id, cards.front, cards.back, cards.created_at, cards.updated_at, cards.position, ucs.state, ucs.due").
    Joins("LEFT JOIN user_card_fsrs ucs ON ucs.user_id = ? AND ucs.card_id = cards.id", userID).
    Where("cards.cardgroup_id = ? AND (ucs.due IS NULL OR ucs.due <= ?)", cardgroupID, now).
    Order("COALESCE(ucs.due, cards.created_at) ASC, cards.position ASC, cards.id ASC").
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

The gotcha is not limited to LEFT JOIN columns. It bites **any** scan target
that embeds a `TableName()`-carrying row struct to ride an extra projected
column alongside it — including a **correlated-subquery aggregate**. The
catalog page query `FindPublishedPage` (`master_cardgroup.go`) projects
`mcg.*, (SELECT COUNT(*) …) AS card_count` and originally scanned into a
`gormMasterCatalogRow` that embedded `gormMasterCardgroup`. The `card_count`
field mapped, but every embedded column (`id`, `name`, `status`, …) scanned to
its zero value, so the query "returned rows" whose IDs were all `""`. The fix
is the same flatten: a flat struct listing each `master_cardgroups` column plus
`card_count`, with a `toGorm()` method that rebuilds `gormMasterCardgroup` so
the domain conversion stays in one place. Using `.Table("master_cardgroups AS
mcg")` does **not** save you — the explicit table sets the FROM clause, but the
column-to-field binding still resolves against the embedded type's schema.

This failure is invisible to usecase unit tests that mock the repository — only
an integration test against a real Postgres exercises the GORM scan path and
catches the blanked columns. Pair any new embedded-extra-column scan target with
a repository integration test that asserts a populated primary key, not just the
extra column.

**Reference:** `backend/internal/repository/card.go` — `dueCardRow` (flat,
LEFT JOIN variant) and `findDueCardsOn`; `backend/internal/repository/master_cardgroup.go`
— `gormMasterCatalogRow` (flat, correlated-COUNT variant) and `FindPublishedPage`.
