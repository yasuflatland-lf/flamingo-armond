# Consumer-defined interface across package boundaries (domain → service)

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.
> For the usecase → repository variant see
> [`docs/backend/library-gotchas/consumer-defined-narrow-repo-interface.md`](../library-gotchas/consumer-defined-narrow-repo-interface.md).

## Why

The `go-arch-lint` invariant forbids `domain → domain/service` imports
(see [`backend/.go-arch-lint.yml`](../../../backend/.go-arch-lint.yml) and
[`.claude/rules/backend-layering.md`](../../../.claude/rules/backend-layering.md)).
`domain/service` depends on `domain`, not the other way around. Yet
`UserCardFSRS.ApplyRating` needs to call a scheduler — a calculation that lives in
`domain/service`.

Putting `ApplyRating`'s logic inline in the aggregate would duplicate the
FSRS algorithm. Calling `service.FSRSScheduler` directly would create a forbidden
import cycle. A global function pointer would be implicit and untestable.

## What

The aggregate declares a minimal interface in its own file:

```go
// domain/user_card_fsrs.go

// FSRSScheduler is the consumer-defined interface for the FSRS scheduler.
// *service.FSRSScheduler satisfies it implicitly.
type FSRSScheduler interface {
    Apply(state FSRSState, rating Rating, now time.Time) FSRSState
}

func (u *UserCardFSRS) ApplyRating(scheduler FSRSScheduler, rating Rating, now time.Time) error {
    if !rating.IsValid() {
        return eris.Errorf("user_card_fsrs: invalid rating %d", rating)
    }
    u.State = scheduler.Apply(u.State, rating, now)
    u.UpdatedAt = now
    return nil
}
```

`*service.FSRSScheduler` has an `Apply` method with the matching signature and
satisfies `domain.FSRSScheduler` implicitly — Go's structural typing requires no
change to the service package. The dependency arrow stays correct: `service`
depends on `domain`; `domain` has no import of `service`.

The caller (usecase layer) passes the concrete `*service.FSRSScheduler` at the
call site:

```go
// usecase/swipe.go (simplified)
if err := uc.Apply(scheduler, rating, now); err != nil { ... }
```

## Relationship to the usecase→repository variant

The [`consumer-defined-narrow-repo-interface`](../library-gotchas/consumer-defined-narrow-repo-interface.md)
pattern (usecase level) and this pattern (domain level) are the same technique
applied at different layers. In both cases the consumer declares the minimal
interface it needs; the concrete implementation satisfies it without importing back.
The domain-level variant is stricter because the forbidden import (`domain →
domain/service`) is architecturally enforced by `go-arch-lint`, not just a
best-practice guideline.
