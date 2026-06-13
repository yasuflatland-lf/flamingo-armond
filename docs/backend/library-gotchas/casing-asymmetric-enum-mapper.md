# Casing-asymmetric enum: persisted lowercase vs wire uppercase needs explicit mappers

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A domain enum that is **persisted in one casing** and **exposed on the wire in
another** has two distinct string spaces that share no values. They cannot be
`==` compared or pointer-cast into each other; the conversion must run through a
pair of explicit mappers, and the two directions are deliberately asymmetric.

`LearnDisplayMode` is the worked example:

| Layer | Type / values | Source |
|---|---|---|
| Domain (persisted) | `domain.LearnDisplayFlipToReveal = "flip_to_reveal"`, `domain.LearnDisplayAlwaysVisible = "always_visible"` | `backend/internal/domain/learn_display_mode.go` |
| Wire (GraphQL) | `model.LearnDisplayModeFlipToReveal = "FLIP_TO_REVEAL"`, `model.LearnDisplayModeAlwaysVisible = "ALWAYS_VISIBLE"` | `backend/graph/model/models_gen.go`, `schema/learn_display_mode.graphql` |

Because the underlying strings differ (`flip_to_reveal` ≠ `FLIP_TO_REVEAL`), the
same-underlying-type pointer cast used for matching-casing newtypes (see
[`same-underlying-type-pointer-cast.md`](../ddd-patterns/same-underlying-type-pointer-cast.md))
does **not** apply. Two `switch`-based mappers in
`backend/graph/resolver/mapper.go` bridge the gap.

## Read direction (domain → wire): exhaustive switch with a fail-safe default

`toLearnDisplayModeModel(domain.LearnDisplayMode) model.LearnDisplayMode` is an
explicit `switch` over every domain value, mirroring the style of
`toCEFRLevelModel` in the same file — **not** an `if/else` funnel that silently
coerces unknowns. The `default` arm returns the flip fallback rather than an
error, because a read path must stay non-fatal during rolling deploys or an
unforeseen schema extension; the DB `CHECK` constraint and `ParseLearnDisplayMode`
both keep unknown values from reaching this branch in production. The fallback is
documented inline as deliberately unreachable, not as a guess.

## Write direction (wire → domain): error on unknown

`fromLearnDisplayModeModel(model.LearnDisplayMode) (domain.LearnDisplayMode, error)`
returns an `eris.Errorf` for any unrecognized value. This is **defense-in-depth**:
gqlgen's generated `LearnDisplayMode.UnmarshalGQL` already rejects any value not in
the schema enum before the resolver runs, so the error branch is reachable only via
a schema mismatch or a client that bypasses enum validation. The resolver maps that
error to `gqlerr.Internal` (not `BAD_USER_INPUT`) because the fault is structural,
not a correctable user mistake — see
`backend/graph/resolver/learn_display_mode.resolvers.go`.

The asymmetry is intentional: the read path cannot fail a query over a value the DB
already accepted, so it falls back; the write path has no value the user could
legitimately supply that the GraphQL layer did not already validate, so an unknown
is a bug worth surfacing.

## A table-driven round-trip test pins both casings

A small test asserts `from(to(d)) == d` for every domain member, which catches a
casing typo in either mapper at test time without a running database:

```go
got, err := fromLearnDisplayModeModel(toLearnDisplayModeModel(tc.domainMode))
// require err == nil && got == tc.domainMode for each domain enum member
```

See `TestLearnDisplayModeMapper_RoundTrip` in
`backend/graph/resolver/mapper_test.go`. The round-trip is the cheapest guard
against the two spellings drifting: change either constant's literal and the test
fails.

## Related, distinct angles

- [`gqlgen-acronym-enum-type-vs-method-casing.md`](gqlgen-acronym-enum-type-vs-method-casing.md)
  — gqlgen casing of the generated *type* vs *resolver method* (`CEFRLevel` vs
  `CefrLevel`). That is about generated-identifier casing within one layer, not
  the persisted-vs-wire value mismatch this doc covers.
- [`.claude/rules/pagination.md` § "Server-side design"](../../../.claude/rules/pagination.md#server-side-design)
  (the "Three layers of enums kept in sync" bullet) — `CardOrderBy` carries the
  *same* string values across `model` / `usecase` / `repository` (snake_case column
  names appear only at the repository layer). The layers there differ by package,
  not by value casing, so they pointer-cast freely; `LearnDisplayMode` cannot, which
  is why it needs the explicit mappers above.
