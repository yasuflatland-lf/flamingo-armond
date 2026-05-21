# Defense-in-depth aggregate-method errors classify as INTERNAL, not BAD_USER_INPUT

> Part of the [error-wrapping conventions](../../../.claude/rules/error-wrapping.md) rules.

## Why

A field-validation sentinel can appear on two paths in the same usecase:

1. **Validation channel** (the user-input boundary): `domain.ParseX(in)` returns
   the sentinel because the user supplied a bad value. The usecase translates it
   via `liftValidationErr(translateXErr(err))` to `ucerr.NewValidationError(...)`,
   surfacing on the wire as `BAD_USER_INPUT` with the field name.
2. **Defense-in-depth path** (the aggregate behaviour method): `aggregate.UpdateX(parsedVO)`
   returns the same sentinel because the parsed VO turned out to be the zero value.
   By construction this is unreachable in production — `ParseX` upstream rejects
   the zero value first. If the path *does* fire, a programmer has bypassed the
   upstream gate.

The temptation is to route both paths through `translateXErr` "for consistency".
This is wrong: the two paths represent different failure modes and must classify
differently.

- **Validation channel** — the user gave bad input. The client receives
  `BAD_USER_INPUT` and the user can correct it.
- **Defense-in-depth path** — a programmer caller violated the aggregate's
  invariant after the gate. The client receives `INTERNAL` because the situation
  is a server-side bug, not a user-input problem. The Application layer logs the
  wrapped error chain so the bug can be diagnosed.

Collapsing both into `BAD_USER_INPUT` lies to the user: they cannot fix the input
because the input was already valid. Collapsing both into `INTERNAL` hides genuine
validation errors that the user could fix. The classification must match the
failure mode, not the sentinel identity.

## What

The usecase wraps a defense-in-depth aggregate-method return with `eris.Wrap`,
not with the validation classifier:

```go
// backend/internal/usecase/card.go — CardUsecase.Update (excerpt)

// UpdateFront/UpdateBack errors below are routed through eris.Wrap, not
// translateCardErr: ParseCardText (called immediately inside each guard)
// already returns the sentinel for empty/zero input on the validation
// channel. If UpdateFront/UpdateBack still rejects the parsed VO, the
// invariant has been violated by a programmer error, not bad user input.
// INTERNAL is the honest classification — surfacing as BAD_USER_INPUT
// would mislead the client.
if in.Front != nil {
    front, err := domain.ParseCardText(*in.Front, domain.ErrCardFrontRequired, domain.ErrCardFrontTooLong)
    if err != nil {
        // Validation channel: surfaces as BAD_USER_INPUT.
        info, perr := liftValidationErr(translateCardErr(err))
        if perr != nil {
            return UpdateCardOutcome{}, perr
        }
        return UpdateCardOutcome{Validation: info}, nil
    }
    // Defense-in-depth: surfaces as INTERNAL.
    if err := existing.UpdateFront(front); err != nil {
        return UpdateCardOutcome{}, eris.Wrap(err, "usecase: card: update front")
    }
    s := existing.Front.String()
    patch.Front = &s
}
```

The two-segment wrap prefix `usecase: card: update front` follows the canonical
layer prefix convention (see [`.claude/rules/error-wrapping.md`](../../../.claude/rules/error-wrapping.md#canonical-layer-prefix)).

## How to recognise the pattern at review time

A defense-in-depth path is identifiable by three structural cues, **all of which
must hold**:

1. The aggregate method has a non-nil error signature returning the same sentinel
   as the upstream parser.
2. The usecase calls the parser first and the aggregate method second, on the same
   input.
3. The aggregate method's docstring explicitly describes itself as defense-in-depth
   (e.g. "The zero-value guard is defense-in-depth: production callers parse the
   input through `ParseX` before reaching this method").

If a method satisfies all three, its error path should classify as `INTERNAL` via
`eris.Wrap`. If only some hold (e.g. an aggregate method that takes a raw string
and is the *only* validation site), the aggregate-method path is the validation
channel and should route through the classifier.

## Comparison with the boundary-gate pattern

A related but distinct pattern is documented in
[`boundary-gate-replaces-domain-recheck.md`](../ddd-patterns/boundary-gate-replaces-domain-recheck.md):
when an invariant has a boundary gate, the aggregate's `Validate()` can drop the
re-check entirely. That pattern *removes* the redundant invariant. The
defense-in-depth pattern *keeps* the redundant invariant but classifies the
redundant path as `INTERNAL`.

The choice between them:

- **Drop the re-check** when the invariant has no defensive value (e.g.
  `Card.CardgroupID` is a user-input field gated at the boundary; an unreachable
  re-check has zero security weight).
- **Keep the re-check as defense-in-depth** when the invariant is an
  *aggregate-state* invariant the type enforces structurally (e.g. `Card.Front`
  must never be the zero `CardText`). A zero-value `Card.Front` corrupts the
  aggregate; the re-check stops the corruption from being committed even if the
  upstream parser is bypassed.

The two patterns are not in conflict — they apply to invariants of different
shapes.

## Reference

- `backend/internal/domain/card.go` — `UpdateFront`/`UpdateBack` with
  defense-in-depth zero-value guard.
- `backend/internal/usecase/card.go` — `CardUsecase.Update` wraps the
  defense-in-depth path with `eris.Wrap` and the validation channel with
  `translateCardErr`.
- Issue #213 — introduced this pattern for `Card.Front` / `Card.Back`.
- [`docs/backend/ddd-patterns/boundary-gate-replaces-domain-recheck.md`](../ddd-patterns/boundary-gate-replaces-domain-recheck.md) —
  the complementary pattern that drops the re-check.
- [`docs/backend/error-wrapping/application-presentation-error-responsibility.md`](application-presentation-error-responsibility.md) —
  the usecase is consumer-agnostic; the presentation layer maps errors to wire
  format. The classification choice belongs to the usecase, not the presentation
  layer.
