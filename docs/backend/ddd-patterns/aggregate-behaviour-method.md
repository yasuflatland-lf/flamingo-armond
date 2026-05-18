# Aggregate behaviour methods: promoting anemic-domain logic

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

An "anemic domain" puts all logic in usecases: the domain model is a data carrier
(structs with no methods), and every caller reimplements the same invariant. The
costs:

- **Duplication.** An invariant checked in three usecases is three places to break
  independently.
- **Coupling.** Callers accumulate knowledge of internal domain rules they should not
  need to understand.
- **Testability.** Unit tests for the invariant live in the usecase test file instead
  of in a focused domain test.

Behaviour methods let the type enforce its own invariants and expose the validated
state via a single API surface.

## What (three promotions in issue #182)

### `Role.IsSystem()` (issue #182 item #14, #15)

Before: `usecase/admin_role.go` contained a local `isSystemRole(name string) bool`
check comparing the string against hardcoded `"admin"` and `"general"` literals.

After: `domain/role.go` owns the set and the method:

```go
var systemRoleNames = map[string]struct{}{
    AdminRoleName:   {},
    GeneralRoleName: {},
}

func (r Role) IsSystem() bool {
    _, ok := systemRoleNames[r.Name]
    return ok
}
```

Any usecase that receives a `*domain.Role` calls `role.IsSystem()` — the check is
not repeated per-caller.

### `Rating.IsValid()` (issue #182 item #16)

Before: rating bounds were checked inline in the swipe usecase via an ad-hoc range
comparison.

After: `domain/rating.go` owns the boundary:

```go
func (r Rating) IsValid() bool {
    return r >= RatingAgain && r <= RatingEasy
}
```

`UserCardFSRS.ApplyRating` calls `rating.IsValid()` before delegating to the
scheduler, so an invalid rating is caught at the aggregate boundary rather than
reaching the service layer.

### `UserCardFSRS.ApplyRating()` (issue #182 item #18)

Before: `usecase/swipe.go` built a `UserCardFSRS{...}` struct literal and called
`scheduler.Apply` inline, producing a partial aggregate state that could diverge
from the constructor invariants if any field was missed.

After: the aggregate owns the state-transition logic:

```go
func (u *UserCardFSRS) ApplyRating(scheduler FSRSScheduler, rating Rating, now time.Time) error {
    if !rating.IsValid() {
        return eris.Errorf("user_card_fsrs: invalid rating %d", rating)
    }
    u.State = scheduler.Apply(u.State, rating, now)
    u.UpdatedAt = now
    return nil
}
```

The usecase passes the concrete scheduler; the aggregate updates its own fields.
`UpdatedAt` is always stamped, and the aggregate's invariants are maintained in
one place.
