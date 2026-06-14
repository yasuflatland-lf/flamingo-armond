# Typed bare-newtype IDs for the authorization-confusable pair

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

Two authorization predicates take the *same* primitive shape — `func(string) bool` —
and answer structurally identical questions about a different ID space:

- `Cardgroup.IsOwnedBy(userID)` — does this user own this cardgroup?
- `Card.BelongsToCardgroup(cardgroupID)` — is this card in this cardgroup?

When both arguments are `string`, the compiler cannot tell the two ID spaces
apart. A caller that transposes a user id and a cardgroup id (or feeds an owner
id into the membership check) compiles cleanly and fails open at runtime — the
worst failure mode for an authorization gate. The risk is highest where two
same-typed parameters sit adjacent in one signature.

The fix is to make the two ID spaces distinct *types*:

```go
// backend/internal/domain/id_types.go
type CardgroupID string
type UserID string
```

`Cardgroup.IsOwnedBy(userID UserID)` and `Card.BelongsToCardgroup(cardgroupID CardgroupID)`
now reject a transposed argument at compile time. The type's only job is
compile-time ID-space tagging.

## Why bare — no `Parse` constructor

These are deliberately **bare** string newtypes with **no** `Parse<Type>`
constructor, which is the opposite of `RoleName` / `DisplayName` / `CardText`
(see [`value-object-parse-pattern.md`](./value-object-parse-pattern.md) and
[`zero-value-docstring-on-string-newtypes.md`](./zero-value-docstring-on-string-newtypes.md)).
The difference is that an ID carries **no domain-authored invariant**:

- **UUID validity is already guaranteed upstream.** `domain.NewID` produces a
  UUID v7 (`backend/internal/domain/ids.go`), and the persisted column is a
  `uuid` type. The domain is *not* the authority on UUID format — a `Parse` that
  re-validated the format would duplicate a guarantee the generator and the DB
  column already enforce.
- **A format `Parse` would mis-classify the error.** Turning a malformed
  client-supplied id into a `BAD_USER_INPUT` validation error is wrong: a stale
  or malformed id must **collapse to not-found** at the lookup, not surface as a
  validation error. Emitting a validation error here would also break the
  non-disclosure-collapse gate (see
  [`notfound-collapse-non-disclosure.md`](./notfound-collapse-non-disclosure.md)),
  which requires unknown and hidden ids to be indistinguishable.
- **A `Parse` would contradict the opaque-handle contract.** Clients treat ids
  (and cursors) as opaque handles they pass back unchanged; the server may
  change the encoding without breaking them. A domain-side format check freezes
  an encoding the contract says is the server's to change.

The "weak VO" gap that `Parse` usually closes — `CardgroupID("")` is
constructible by any caller — is neutralized here without a parser, by two
mechanisms already in place:

1. The empty-→`false` guards inside the predicates themselves:
   `IsOwnedBy` returns `userID != "" && c.OwnerID == userID`;
   `BelongsToCardgroup` returns `cardgroupID != "" && c.CardgroupID == cardgroupID`.
   An empty id can never satisfy an ownership or membership check.
2. The zero-value docstring convention (see
   [`zero-value-docstring-on-string-newtypes.md`](./zero-value-docstring-on-string-newtypes.md)):
   the type doc states the zero value `""` is invalid and that ids are
   constructed by a direct cast at boundaries.

## Scope boundary — type only the authz-confusable pair

Only `UserID` and `CardgroupID` are typed. `CardID`, `RoleID`, and the
master-aggregate ids stay raw `string` **by design** — they are not
authorization-confusable, and typing them would fan out through the master
mappers and every `FindByID` caller with no authz payoff. The `Card.ID` field,
for instance, stays `string` (`backend/internal/domain/card.go`); only
`Card.CardgroupID` is typed.

## Depth boundary — type the helper signatures, not the shared interface

Apply the type where the confusion lives, stop where it ripples without benefit.

**Typed** (the transposition-risk surface):

- The domain struct fields — `Cardgroup.ID` / `Cardgroup.OwnerID`,
  `Card.CardgroupID` (`backend/internal/domain/cardgroup.go`,
  `backend/internal/domain/card.go`).
- The aggregate method signatures — `IsOwnedBy(userID UserID)`,
  `BelongsToCardgroup(cardgroupID CardgroupID)`.
- The authorization **helper** signatures where two same-type params sit
  adjacent. `authorizeCardgroupOrBadInput` and
  `authorizeCardgroupOrUnauthenticated` both take
  `(id domain.CardgroupID, userID domain.UserID)`
  (`backend/internal/usecase/ownership.go`) — this adjacency is the exact
  transposition site the typing protects.
- The usecase input field `HandleSwipeInput.CardgroupID`
  (`backend/internal/usecase/swipe.go`), which flows straight into the gate.

**Not typed** (rippling boundaries, cast across instead):

- The shared repository surface — both the usecase-narrow
  `CardgroupOwnershipFinder.FindByID` (`backend/internal/usecase/ownership.go`)
  and the underlying `repository.CardgroupRepository.FindByID`
  (`backend/internal/repository/cardgroup.go`) keep
  `FindByID(ctx, id string)`. Typing them would ripple to every `FindByID`
  caller for no authz gain. The helper casts `string(id)` at that single
  boundary instead.
- Non-authz usecase input fields — `CreateCardInput.CardgroupID`,
  `CardConnectionInput.CardgroupID`, `ImportCardsInput.CardgroupID`
  (`backend/internal/usecase/card.go`) stay raw `string`; the usecase casts
  `domain.CardgroupID(in.CardgroupID)` at the call into the gate or aggregate
  constructor.
- The repository row structs — `gormCardgroup.OwnerID` stays `string`
  (`backend/internal/repository/cardgroup.go`). The cast lives in the
  row↔domain mappers: `cardgroupToDomain` widens with
  `domain.CardgroupID(g.ID)` / `domain.UserID(g.OwnerID)`, and `cardgroupToRow`
  narrows with `string(cg.ID)` / `string(cg.OwnerID)`. Both directions are the
  same-underlying-type conversion (see
  [`same-underlying-type-pointer-cast.md`](./same-underlying-type-pointer-cast.md)
  for why this cast is legal and needs no helper).

The auth-subject boundary is the third cast site: the usecase widens the
JWT subject with `domain.UserID(user.Sub)` at each call into a typed predicate
or helper (`backend/internal/usecase/card.go`,
`backend/internal/usecase/cardgroup.go`, `backend/internal/usecase/swipe.go`).

## Worked example — issue #438

The pattern was introduced to harden the two ownership predicates that had
identical `func(string) bool` shapes. After the change:

- `Cardgroup.IsOwnedBy(domain.UserID(user.Sub))` and
  `Card.BelongsToCardgroup(domain.CardgroupID(cardgroupID))` cannot be called
  with each other's id type.
- `authorizeCardgroupOrBadInput(ctx, repo, domain.CardgroupID(in.CardgroupID), domain.UserID(user.Sub))` —
  the two adjacent ids are now distinct types, so transposing them fails to
  compile.
- The repository interface and row structs were left raw `string`; the casts
  live only at the three boundaries (usecase input, auth subject, row mapper).

## Reference

- `backend/internal/domain/id_types.go` — `type CardgroupID string` / `type UserID string`, bare, no `Parse`.
- `backend/internal/domain/cardgroup.go` — `Cardgroup.IsOwnedBy(userID UserID)`, typed `ID` / `OwnerID` fields.
- `backend/internal/domain/card.go` — `Card.BelongsToCardgroup(cardgroupID CardgroupID)`, typed `CardgroupID` field, raw `string` `ID`.
- `backend/internal/usecase/ownership.go` — the typed helper signatures and the `string(id)` cast at the `FindByID` boundary.
- `backend/internal/repository/cardgroup.go` — raw-`string` row struct + the `cardgroupToDomain` / `cardgroupToRow` casts.
