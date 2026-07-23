# Boundary gate replaces domain re-check

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

A domain aggregate's enforced constructor (e.g. `NewCard`) is the temptation to
centralise every invariant in one place: "validate everything every time, then
it is impossible to forget". The problem with double-checking an invariant that
the usecase boundary already enforces is twofold:

- **Dead code.** If the only callers of the aggregate constructor flow through
  the boundary gate, the domain-side check never fires for any non-malicious
  input. It is exercised only by domain unit tests that construct invalid
  shapes deliberately.
- **Drift risk.** The boundary gate and the domain re-check express the same
  invariant in two languages (e.g. `ucerr.NewValidationError("cardgroupId", ...)`
  at the boundary, `eris.New("card: cardgroup id is required")` in the domain).
  When the message text on one side changes, the other drifts unnoticed.

The right shape is: the boundary enforces caller-input invariants; the
aggregate enforces internal-state invariants. A field that originates in
user-supplied input and is gated at the boundary should not be re-checked in
the domain.

## What — `Card.CardgroupID`

`Card.CardgroupID` originates exclusively from caller input (a GraphQL mutation
argument). Every usecase entry point that mutates a `Card` runs
`authorizeCardgroupOrBadInput` before constructing the aggregate:

```go
// backend/internal/usecase/card.go — CardUsecase.Create
if err := authorizeCardgroupOrBadInput(ctx, u.cardgroupRepo, in.CardgroupID, user.Sub); err != nil {
    return CreateCardOutcome{}, err
}
// ... Card is constructed only after the gate has passed.
```

```go
// backend/internal/usecase/ownership.go
func authorizeCardgroupOrBadInput(ctx context.Context, repo CardgroupOwnershipFinder, id, userID string) error {
    cg, err := repo.FindByID(ctx, id)
    if err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            return ucerr.NewValidationError("cardgroupId", "cardgroup not found")
        }
        return eris.Wrap(err, "usecase: authorize cardgroup: find by id")
    }
    if !cg.IsOwnedBy(userID) {
        return ucerr.ErrUnauthenticated
    }
    return nil
}
```

An empty `CardgroupID` reaches `repo.FindByID`, where PostgreSQL reports the
malformed UUID bind as SQLSTATE `22P02`. The repository classifies that error as
`ErrNotFound`, which the gate translates into
`BAD_USER_INPUT(field=cardgroupId)`. The GraphQL caller sees a typed validation
error before any `Card` is constructed.

A domain-side `if c.CardgroupID == ""` check inside `NewCard` would
have been unreachable for every production caller. The check was removed
along with its `ErrCardCardgroupIDRequired` sentinel; the aggregate's
docstring documents the boundary contract instead:

```go
// backend/internal/domain/card.go
// Card is an aggregate root. ...
//
// CardgroupID ownership is enforced at the usecase boundary via
// authorizeCardgroupOrBadInput before a Card is constructed; the domain
// aggregate therefore does not re-check CardgroupID presence in NewCard.
type Card struct { ... }
```

## Front and Back stay enforced in NewCard

`NewCard` (and its `MasterCard` sibling `NewMasterCard`) validates `Front` and
`Back` through `ParseCardText`. Those fields are not gated at the usecase
boundary; `ParseCardText` is the single source of truth for the grapheme-bound +
non-empty invariants. The enforced constructor is the single construction path
for a new `Card` — an invalid `Front`/`Back` cannot produce a constructed
aggregate — so the invariant is enforced exactly once, in the constructor,
rather than re-checked by a separate `Validate()` method (which had zero
production callers and was removed).

The pattern is: each invariant has a single enforcement site. If the
boundary enforces it, the domain skips it. If the domain owns it (because
no boundary gate has it), the domain enforces it.

## When the pattern does NOT apply

- **Cross-aggregate invariants** that span fields with different origins.
  Example: a `Card.UpdatedAt` that must not be earlier than `Card.CreatedAt`
  is a domain-internal consistency check; no caller-facing boundary owns it.
- **Defense in depth for sensitive operations** (authorization, money,
  cryptographic state) where a redundant check is cheap insurance against
  a future caller bypassing the gate. The `Card.CardgroupID` case is plain
  user input with no security weight beyond what the gate already provides.

## Reference

- `backend/internal/domain/card.go` — `NewCard` enforces `Front`/`Back` but not `CardgroupID` presence; docstring notes the boundary contract.
- `backend/internal/usecase/ownership.go` — `authorizeCardgroupOrBadInput`, the gate that owns the invariant.
- `backend/internal/usecase/card.go` — `Create` / `Update` call the gate before constructing the aggregate.
