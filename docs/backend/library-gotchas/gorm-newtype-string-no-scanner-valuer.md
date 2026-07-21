# GORM v1 round-trips underlying-string newtypes without Scanner/Valuer

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## The reflective fallback

`database/sql` resolves a value to be sent to the driver in two steps:

1. Check whether the value implements `driver.Valuer`. If yes, call `Value()`.
2. Fall back to a reflective converter (`database/sql/driver.DefaultParameterConverter`).
   For any type whose `reflect.Kind` is `String`, the converter returns the
   underlying string verbatim. Likewise for `Scan`: a `*string` target accepts
   the column's text value via the reflective path.

Consequently, a Go newtype defined as `type RoleName string` (or `type CardText string`,
`type DisplayName string`) round-trips through a GORM v1 query against a `text`
column **without** an explicit `Scanner`/`Valuer` pair — provided the GORM row
struct types the column as the underlying primitive (`string`), not as the newtype.

In this repository the gorm row structs intentionally use primitives at the
column boundary:

```go
// backend/internal/repository/role.go
type gormRole struct {
    ID   string `gorm:"column:id;primaryKey;type:uuid"`
    Name string `gorm:"column:name"`     // <- string, not RoleName
}

// backend/internal/repository/card.go
type gormCard struct {
    ID          string    `gorm:"column:id;primaryKey;type:uuid"`
    CardgroupID string    `gorm:"column:cardgroup_id"`
    Front       string    `gorm:"column:front"`   // <- string, not CardText
    Back        string    `gorm:"column:back"`
    CreatedAt   time.Time `gorm:"column:created_at"`
    UpdatedAt   time.Time `gorm:"column:updated_at;->"` // read-only: DB trigger owns it
}
```

The VO conversion happens at the boundary in the `*ToDomain` helper:

```go
// backend/internal/repository/card.go — cardToDomain
return &domain.Card{
    ID:          row.ID,
    CardgroupID: row.CardgroupID,
    Front:       domain.CardText(row.Front),   // <- straight cast at boundary
    Back:        domain.CardText(row.Back),
    // ...
}
```

## When Scanner/Valuer are dead code

If the gorm row struct already types the column as a primitive (`string`,
`*string`), adding `Scan`/`Value` methods to the newtype produces dead code:
the reflective converter never reaches the newtype because the row struct's
field is the primitive, and the conversion to the newtype happens *after* the
DB round-trip.

A Scanner/Valuer pair on the newtype is only load-bearing when the gorm row
struct types the column with the VO directly (e.g. `Name RoleName \`gorm:"column:name"\``).
That introduces a different design trade-off (the gorm row becomes
domain-typed) and is not the pattern this repository uses.

## The Bio counter-example — boundary helper, not Scanner/Valuer

`Bio` is a struct VO (`type Bio struct { value *string }`), not a string newtype.
A struct VO does not benefit from the reflective fallback, so the conversion
needs an explicit boundary helper. The repository writes the underlying `*string`
verbatim into the gorm row's `*string` column field, and reads back via
`domain.BioFromPtr(g.Bio)`:

```go
// backend/internal/repository/user.go — userToDomain
return &domain.User{
    // ...
    Bio: domain.BioFromPtr(g.Bio),
    // ...
}
```

`BioFromPtr` is wired by the repository read path; it is the load-bearing
counterpart to the dead Scanner/Valuer pair. The dead-code rule (delete unwired
helpers; see [`docs/backend/ddd-patterns/helpers-introduced-but-not-wired-must-be-deleted.md`](../ddd-patterns/helpers-introduced-but-not-wired-must-be-deleted.md))
is keyed on wired-ness, not symbol kind: `BioFromPtr` and the never-wired
`Scan`/`Value` methods were introduced together; only the former survived.

## Method-name collision when reserving the `Value` slot

`Bio.Value() *string` was the original accessor for the underlying pointer. To
support a future `driver.Valuer` interface (`Value() (driver.Value, error)`),
the accessor would have to be renamed — Go does not allow two methods with the
same name and different signatures on the same receiver.

The accessor was renamed to `Ptr()` and the `Value` slot was left free:

```go
// backend/internal/domain/bio.go
func (b Bio) Ptr() *string { ... }
func (b Bio) IsSet() bool  { return b.value != nil }
```

The lesson: when designing a struct VO whose underlying value is a pointer
or interface that future code may want to expose via a `database/sql` or
`json` interface (`Value`, `MarshalJSON`, `UnmarshalJSON`), avoid naming the
accessor `Value` up front. `Ptr`, `Get`, `Unwrap`, or the field name itself
(`BioText`, `BioPtr`) all keep the interface slots free. The same applies to
`Scan` (avoid an accessor named `Scan`).

This is a soft constraint: when the future interface actually arrives the
rename is a mechanical refactor. The cost is the cascade of call-site updates
(here: ~10 call sites across repo + tests + resolver) and any docs/comments
that name the old method. Pay the cost eagerly if the interface is anticipated;
otherwise rename when the need is concrete.

## Reference

- `backend/internal/repository/role.go` — `gormRole.Name string` + `domain.RoleName(g.Name)` at the boundary.
- `backend/internal/repository/card.go` — `gormCard.Front/Back string` + `domain.CardText(row.Front)` in `cardToDomain`.
- `backend/internal/repository/user.go` — `gormUser.DisplayName/Bio *string` + `(*domain.DisplayName)(g.DisplayName)` and `domain.BioFromPtr(g.Bio)` in `userToDomain`.
- `backend/internal/domain/bio.go` — `Bio.Ptr()` accessor with the `Value` slot reserved for future `driver.Valuer`.
