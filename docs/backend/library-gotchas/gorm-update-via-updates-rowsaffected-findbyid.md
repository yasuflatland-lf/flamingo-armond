# GORM `Updates(map)` + `RowsAffected == 0` + `FindByID` for race-free single-field updates

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`Take + Save` overwrites every column on the in-memory struct, so a concurrent
write to a different column on the same row is silently lost. The
`Updates(map[string]any{...}) → RowsAffected == 0 → FindByID` idiom closes
that lost-update window and returns the freshly-stored row in a single
round-trip sequence.

```go
// backend/internal/repository/role.go — roleRepo.Update (lines 170–191)

func (r *roleRepo) Update(ctx context.Context, id, name string) (*domain.Role, error) {
    name = strings.ToLower(strings.TrimSpace(name))

    res := r.db.WithContext(ctx).
        Model(&gormRole{}).
        Where("id = ?", id).
        Updates(map[string]any{"name": name})
    if res.Error != nil {
        if classified := classifyUniqueError(res.Error); classified != nil {
            return nil, classified
        }
        return nil, eris.Wrap(res.Error, "repository: role: update")
    }
    if res.RowsAffected == 0 {
        return nil, ErrRoleNotFound
    }
    role, err := r.FindByID(ctx, id)
    if err != nil {
        return nil, eris.Wrap(err, "repository: role: update: find after update")
    }
    return role, nil
}
```

## Why this shape

1. **`Updates(map[string]any{...})`** writes only the named columns. The SQL
   becomes `UPDATE roles SET name = ? WHERE id = ?` — every other column on the
   row is untouched. `Take + Save` issues `UPDATE roles SET name = ?, created_at
   = ?, ... WHERE id = ?` using whatever was in the struct at `Take` time, so a
   concurrent write to any other column between `Take` and `Save` is silently
   reverted.

2. **`RowsAffected == 0`** is the canonical "no row matched the WHERE clause"
   signal. It is checked *after* `res.Error` so a DB-level error is never
   misclassified as a missing row. The same idiom appears in
   `cardRepo.Update` (`backend/internal/repository/card.go`, lines 443–463) and
   `cardgroupRepo.Update` (`backend/internal/repository/cardgroup.go`, lines
   374–393).

3. **`classifyUniqueError`** handles Postgres error 23505 (unique-constraint
   violation) identically whether the error originates from `Save` or `Updates`
   — no change to caller error-handling is required when switching idioms.

4. **`FindByID` after `Updates`** returns the row as it exists in the database
   after the write, including any trigger-refreshed columns (`updated_at`,
   computed columns). Wrapping the `FindByID` error with the label
   `"repository: role: update: find after update"` keeps logs distinguishable:
   a write-succeeded-but-readback-failed failure surfaces with its own wrap
   prefix rather than looking identical to a write failure.

## Race window the pattern does NOT eliminate

The `Updates → FindByID` sequence introduces a small inner read-after-write
window: if a concurrent delete arrives between the successful `Updates` and the
`FindByID`, the latter returns `ErrRoleNotFound`. The usecase layer must
enumerate both this repo-internal window and any outer usecase-level window in
its TOCTOU comment. See `backend/internal/usecase/admin_role.go` lines 252–256
for the canonical form:

```go
// TOCTOU: between FindByID and roles.Update another admin can delete the row;
// the resulting ErrRoleNotFound is mapped back to BAD_USER_INPUT(field=id)
// rather than INTERNAL. The race window also covers the repository-internal
// re-fetch inside roles.Update (Updates → FindByID), where the same
// concurrent delete surfaces uniformly as ErrRoleNotFound.
```

## What this replaces

The legacy `Take` + mutate-in-place + `Save` pattern has a true lost-update
race: between `Take` and `Save`, any concurrent write that touches a different
column on the same row is silently overwritten by `Save`. `Updates(map)` writes
only the named columns, so the race window is closed at the SQL level.

## Reference

- `backend/internal/repository/role.go` — `roleRepo.Update` (lines 170–191, canonical form)
- `backend/internal/repository/card.go` — `cardRepo.Update` (lines 443–463)
- `backend/internal/repository/cardgroup.go` — `cardgroupRepo.Update` (lines 374–393)
- `backend/internal/repository/role_crud_test.go` — `TestRoleRepository_Update_Success` (line 137), `_NormalizesName` (line 170), `_NotFound` (line 193), `_Duplicate` (line 207)
- `backend/internal/usecase/admin_role.go` — TOCTOU comment enumerating both race windows (lines 252–256)
