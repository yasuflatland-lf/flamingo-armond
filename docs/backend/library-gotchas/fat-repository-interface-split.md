# Fat repository interface split: CRUD vs membership seam

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a repository interface accumulates both entity-level CRUD operations (working
directly on the entity's own table) and join-table membership operations (working on
a many-to-many pivot table), the interface has grown past its natural boundary. The
two method sets have different consumers, different test double requirements, and different
error sentinels, which makes them poor roommates.

## Signals that a split is due

1. The interface has more than ~8 methods.
2. Some methods operate on the entity table (`roles`) while others operate on a join
   table (`user_roles`). The join table typically has its own `gormXxx` struct.
3. The concrete repository file imports `clause.OnConflict` (for the join-table's
   idempotent insert) alongside plain `Find`/`Create`/`Update`/`Delete` GORM calls.
4. Different consumers only call one half: a DataLoader calls `FindByIDs`; an auth
   middleware calls only `HasRole`; a usecase calls only `AssignToUser`/`RevokeFromUser`.

## The split

Separate into two interfaces owned by two files:

```
repository/
  role.go       — RoleRepository: 7 CRUD methods
                  FindByID, FindByName, FindByIDs, Create, Update, Delete, ListAll
  user_role.go  — UserRoleRepository: 6 membership methods
                  HasRole, AssignToUser, RevokeFromUser,
                  ListByUser, ListByUserIDs, CountAdmins
```

Error sentinels (`ErrUserNotFound`, `ErrRoleNotFound`, `ErrRoleDuplicate`) and FK
classifiers (`classifyFKError`) stay in the file that first needs them and are
imported by the other file if shared. Sentinels that signal "the other entity is
missing" are correct on the membership file (e.g. `ErrUserNotFound` is declared on
`role.go` but used from `user_role.go`).

## Pre-write validation helpers must pass through context errors

A helper shared by write operations (e.g. `requireExists` in `user_role.go`) typically
runs a COUNT query to validate existence before the write. When the COUNT returns an
error, the helper must distinguish context errors from infrastructure errors:

```go
func (r *userRoleRepo) requireExists(ctx context.Context, table, id, wrap string, notFound error) error {
    var count int64
    if err := r.db.WithContext(ctx).Table(table).Where("id = ?", id).Count(&count).Error; err != nil {
        if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
            return err // pass through unwrapped — caller's errors.Is(_, context.Canceled) must work
        }
        return eris.Wrap(err, wrap)
    }
    if count == 0 {
        return notFound
    }
    return nil
}
```

Without the context-pass-through, a canceled request returns `eris.Wrap(context.Canceled, wrap)`.
`errors.Is` still detects the wrapped sentinel, but the returned error is no longer the bare
sentinel — it violates the identity-pin rule documented in
[`pin-unwrapped-context-error-with-identity-check.md`](../../backend/error-wrapping/pin-unwrapped-context-error-with-identity-check.md).
The same pass-through applies to DataLoader batch functions that delegate to the
membership repository.

## Post-split wiring checklist

After the interface split, update these sites in dependency order:

1. **DataLoader** (`internal/loader/`): change the parameter type from `RoleRepository`
   to `UserRoleRepository` in the batch function and all `New`/`NewWithXxx`/`Middleware`
   constructors.
2. **auth.Service** (`internal/auth/`): if the service only calls `HasRole`, declare a
   narrow `roleChecker` interface rather than depending on the full `UserRoleRepository`.
   This keeps the auth package free of the membership repository import and lets test
   stubs implement only the one method they exercise.
3. **Usecase narrow interfaces**: split existing narrow interfaces that mixed CRUD lookups
   with membership writes into two (e.g. `adminRoleRepository` for `FindByIDs` and
   `adminUserRoleRepository` for `AssignToUser`/`RevokeFromUser`).
4. **Composition root** (`cmd/server/main.go`): instantiate both concrete repositories and
   pass each to the right consumer.
5. **go-arch-lint**: run `go tool go-arch-lint check --project-path .` — a split that
   introduces a new package component needs a `deps:` entry.

## Verify with grep

After the split the slimmed `role.go` must contain zero join-table references:

```bash
grep -n "user_roles\|AssignToUser\|RevokeFromUser\|ListByUser\|CountAdmin" \
    backend/internal/repository/role.go
```

Both invocations must return zero matches.

## Reference

`backend/internal/repository/role.go` and `backend/internal/repository/user_role.go`
after Issue #203. The split reduced `RoleRepository` from 12 methods to 7 and gave
`UserRoleRepository` its own 6-method surface. The consumer-defined narrow interface
pattern for `auth.Service` is documented separately in
[`consumer-defined-narrow-repo-interface.md`](consumer-defined-narrow-repo-interface.md).
