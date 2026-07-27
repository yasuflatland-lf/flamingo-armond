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

#### The scheduler clamps a backward-reading clock

`ApplyRating` hands `now` to the scheduler unchanged, and the scheduler clamps it:
`service.FSRSScheduler.Apply` raises `now` to `state.LastReview` whenever the
incoming timestamp reads earlier than the stored last review. The guard is the
`if now.Before(state.LastReview)` block at the top of `Apply` in
[`backend/internal/domain/service/fsrs_scheduler.go`](../../../backend/internal/domain/service/fsrs_scheduler.go).

The clamp is a domain safety rule, not an implementation convenience. A backward
clock step — an NTP correction, cross-instance skew — leaves `LastReview` after
`now`, which go-fsrs rejects as an invalid card (`validate.go`). `Apply` panics on
that error rather than widening its single-value signature, so one skewed request
would surface as an INTERNAL GraphQL error instead of a review. Clamping keeps elapsed time
non-negative, at the cost of treating a backward-skewed review as if it happened
at the instant of the previous one. Any reimplementation of the `ApplyRating`
path must reproduce it.

**A clamped apply leaves `State.LastReview > UpdatedAt` in the in-memory
aggregate.** `ApplyRating` stamps `u.UpdatedAt = now` with the *unclamped*
argument while the state it stores back carries the *clamped* `LastReview`, so
the loaded aggregate holds the inversion for the rest of the request. It does not
travel to the row: `updated_at` is database-owned, so the repository never sends
the aggregate's value and the `trg_user_card_fsrs_set_updated_at`
BEFORE INSERT OR UPDATE trigger assigns `now()` on either upsert branch.
The persisted `updated_at` is therefore the database clock, never the skewed
aggregate value. The inversion is tolerated rather than normalised because
nothing reads that ordering: the queue predicates compare `last_review` and `due`
against learn-day boundaries, never against `updated_at`.

### Single-field aggregate mutation (`Cardgroup.Rename`, `Card.UpdateFront`/`UpdateBack`) (issues #212, #213)

**Before**: `CardgroupUsecase.Update` and `CardUsecase.Update` mutated the
persistence patch directly from a parsed VO:

```go
// old shape — patch derived from the local VO, aggregate never mutated
nameStr := name.String()
updated, err := u.repo.Update(ctx, id, repository.CardgroupUpdate{Name: &nameStr})
```

The aggregate (`*domain.Cardgroup`, `*domain.Card`) was loaded via `repo.FindByID`
but never mutated through a behaviour method. A future invariant spanning two
fields — say, `UpdatedAt >= CreatedAt` — would have had no obvious home.

**After**: each aggregate exposes a behaviour method that enforces the
aggregate-state invariant (the field must never become the zero VO). The usecase
mutates `existing` via the method, then derives the persistence patch from the
*mutated aggregate*:

```go
// Cardgroup.Rename — usecase route-through
if err := existing.Rename(name); err != nil {
    return UpdateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: rename")
}
nameStr := existing.Name.String()
updated, err := u.repo.Update(ctx, id, repository.CardgroupUpdate{Name: &nameStr})
```

```go
// Card.UpdateFront — usecase route-through; UpdateBack is symmetric
if err := existing.UpdateFront(front); err != nil {
    return UpdateCardOutcome{}, eris.Wrap(err, "usecase: card: update front")
}
s := existing.Front.String()
patch.Front = &s
```

The two-step "behaviour method → patch derivation" decouples invariant enforcement
from persistence shape; the repository patch DTO never knows about aggregate methods.

**Single-field vs combined method.** For per-field mutation — where the patch DTO
has a `*string` per field that may independently be `nil` — two single-field methods
beat one combined `UpdateText(front, back *CardText)` method. Reasons:

1. The patch's `nil = no change` semantics belong to the usecase layer; the
   aggregate should not interpret per-field nullability.
2. Each single-field method encodes exactly one invariant; combined methods
   accumulate tri-state pointer handling that obscures the per-field invariant.
3. Symmetric methods (`UpdateFront` / `UpdateBack`) document parity and let unit
   tests use a single shape.

**Defense-in-depth zero-value guard.** Each method's body is structurally identical:

```go
func (c *Card) UpdateFront(front CardText) error {
    if front == "" {
        return ErrCardFrontRequired
    }
    c.Front = front
    return nil
}
```

The guard is defense-in-depth: production callers parse the input through
`ParseCardText` before reaching the method. The guard fires only if a future caller
bypasses the parser, which is a programmer error that classifies as `INTERNAL`, not
`BAD_USER_INPUT`. See
[`docs/backend/error-wrapping/defense-in-depth-classification-internal.md`](../error-wrapping/defense-in-depth-classification-internal.md).

`Cardgroup.Rename` follows the identical shape using `CardgroupName` and
`ErrCardgroupNameRequired`.

**`UpdatedAt` is intentionally not stamped by the aggregate methods.** Persistence
(GORM `AutoUpdateTime` on the `UpdatedAt` field) is the canonical source of the
modification timestamp. Stamping `c.UpdatedAt = time.Now()` inside a behaviour
method would couple the aggregate to a clock seam and introduce a second
source-of-truth for the timestamp. Compare `UserCardFSRS.ApplyRating` above, which
does stamp `UpdatedAt` — that case owns the full state-transition including the
timestamp because the FSRS algorithm dictates the exact moment the card state
changes; the simpler rename/field-update cases have no such requirement.
