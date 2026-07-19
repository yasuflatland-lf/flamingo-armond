# View-level value lives in the domain package when domain logic consumes it

> Part of the [DDD patterns](../../../.claude/rules/ddd-patterns.md) rules.

## Why

Not every type in `domain/` is an aggregate. A *view-level value* is a
non-aggregate struct that bundles existing aggregate references with
projection-only fields needed by domain logic. The placement question is
whether the type belongs in the domain package or in a separate
`readmodel`/`view` package.

The deciding signal is *who consumes the type*. When the consumer is a domain
service that already lives in `domain/service/`, hoisting the type to a
separate package introduces an import path for no behaviour change — the
domain service would import a sibling package solely to receive its own
input. Keep the type in `domain/` and document the read-model intent on the
struct.

## What

`DueCard` is the read-model input to `OrderingPolicy.Apply`. It carries a
`*Card` plus the viewer's FSRS `State` and `Due` timestamp:

```go
// backend/internal/domain/due_card.go
package domain

import "time"

// DueCard is a Card paired with the viewer's FSRS state for queueing decisions.
//
// Phase is FSRSPhaseNew when the viewer has no user_card_fsrs row for the card,
// in which case Due is the card's created_at as a stable substitute.
//
// DueCard is not an aggregate; it is a view-level value shared between the
// repository and OrderingPolicy. It lives in the domain package because the
// ordering policy is domain logic and DueCard is its input.
//
// Card must not be nil; downstream consumers (OrderingPolicy.Apply) dereference
// it unconditionally. The Phase invariant (FSRSPhaseNew ↔ Due == Card.CreatedAt)
// is established by the repository and is not enforced at the domain layer
// today; new construction sites must reproduce it.
type DueCard struct {
    Card  *Card
    Phase FSRSPhase
    Due   time.Time
}
```

### "Unseen" is "no FSRS row", not `Due == CreatedAt`

A card is unseen for a viewer exactly when **no `user_card_fsrs` row exists for
that (user, card) pair**. That is the definition consumers must test against; the
repository expresses it as the LEFT JOIN miss — `ucs.due IS NULL` in the new-card
window of `findDueCardsOn`, and a nil `State` column when mapping rows into
`DueCard`.

`Due = Card.CreatedAt` is what the repository *synthesizes* for such a row so the
read-model always carries a stable, orderable timestamp. It is a display
placeholder, not an identity, and the implication runs one way only: an unseen row
always gets `Due == CreatedAt`, but a reviewed card whose scheduled `Due` happens
to land on its `CreatedAt` satisfies the same equality while having an FSRS row.
The `FSRSPhaseNew ↔ Due == Card.CreatedAt` shorthand in the docstring above holds
in the forward direction only; treating the equality as an unseen test
misclassifies that coincidence. Branch on `Phase == FSRSPhaseNew` (which the
repository sets from the same join miss) or query the FSRS row directly.

`OrderingPolicy.Apply` accepts `[]DueCard` and returns `[]*Card` — the
read-model is the *input* type, the aggregate is the *output*. The
repository builds `DueCard` values from a LEFT JOIN of `cards` and
`user_card_fsrs`; the domain service consumes them; no other layer needs to
construct them.

## When to keep in `domain/` vs. extract

| Signal | Verdict |
|---|---|
| Type is consumed by a domain service (`domain/service/*.go`) | Keep in `domain/` |
| Type is consumed only by adapters (repository, GraphQL resolver) | Extract to `readmodel/` or keep in `repository/` |
| Type carries domain-level invariants (e.g. the `Phase → Due` synthesis) | Keep in `domain/` so the invariant comment is co-located |
| Type is a thin projection with no domain semantics | Adapter-side DTO is fine |

The first row applies here: `OrderingPolicy.Apply` is the consumer and lives
in `domain/service/`. The Go import graph then has
`repository → domain` (constructs `DueCard`) and `service → domain`
(consumes `DueCard`), with no extra package introduced. The
go-arch-lint layer model in [`backend-layering.md`](../../../.claude/rules/backend-layering.md)
admits both arrows.

## Caveat — invariants are documented, not enforced

`DueCard` does not have a `ParseDueCard` constructor: the Phase/Due
relationship is established by the repository's LEFT JOIN logic, not by a
domain-side validator. The docstring is the load-bearing contract. New
construction sites (typically test fixtures and any future migration paths)
must read the docstring and reproduce the synthesis — a row with no FSRS state
gets `Phase = FSRSPhaseNew` and `Due = Card.CreatedAt`. This is the same
discipline as [zero-value docstring on string newtypes](zero-value-docstring-on-string-newtypes.md),
applied to a struct value: a comment, not a compiler check, because no
single trust-boundary parse exists for the read-model.

## Reference

- `backend/internal/domain/due_card.go` — `DueCard` declaration with the
  read-model docstring.
- `backend/internal/domain/service/due_card_ordering.go` — `OrderingPolicy.Apply`
  consumes `[]DueCard`.
- `backend/internal/repository/card.go` — `findDueCardsOn` constructs
  `DueCard` from the LEFT JOIN result.
