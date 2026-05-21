# Resolver-injected usecase: interface field unlocks unit tests for outcome-union guards

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.
> Closely related to [`docs/backend/error-wrapping/outcome-union-enforcement.md`](../error-wrapping/outcome-union-enforcement.md)
> and [`docs/backend/library-gotchas/xor-invariant-outcome-struct.md`](xor-invariant-outcome-struct.md).

## Why

The [XOR-invariant outcome struct](xor-invariant-outcome-struct.md) pattern leaves
the resolver with a defensive guard: when every variant pointer in the returned
`<Op>Outcome` is `nil` and the `error` slot is also `nil`, the resolver MUST
convert the impossible state to `gqlerr.Internal`. The guard is structural —
it documents the producer/consumer contract — and the only reliable way to
prove it stays wired up correctly across refactors is a unit test that drives
the usecase to return a zero-value outcome.

Whether that unit test is **possible at all** depends on a single design choice
in `backend/graph/resolver/resolver.go`: the resolver's usecase field is either
a concrete struct pointer (`*usecase.CardgroupUsecase`) or an interface
(`usecase.LastViewedCardgroupUsecase`). The two shapes produce
**asymmetric testability** for the nil-variant guard.

## What

The resolver struct in `backend/graph/resolver/resolver.go` mixes both shapes:

```go
type Resolver struct {
    CardgroupUC           *usecase.CardgroupUsecase           // concrete struct pointer
    LastViewedCardgroupUC usecase.LastViewedCardgroupUsecase  // interface
    // ...
}
```

A resolver-package unit test (`backend/graph/resolver/*_test.go`) can substitute
the dependency only when the field is an interface. The interface case yields
the test in `last_viewed_cardgroup_resolver_test.go`:

```go
type mockLastViewedCardgroupUsecase struct {
    setOutcome usecase.SetLastViewedCardgroupOutcome
    setErr     error
}

func (m *mockLastViewedCardgroupUsecase) Set(_ context.Context, _ string) (usecase.SetLastViewedCardgroupOutcome, error) {
    return m.setOutcome, m.setErr
}

func TestSetLastViewedCardgroup_NilVariant_ReturnsInternal(t *testing.T) {
    mock := &mockLastViewedCardgroupUsecase{
        setOutcome: usecase.SetLastViewedCardgroupOutcome{}, // no variant set
    }
    srv := newLastViewedSrv(mock)
    // ... drive the mutation, assert extensions.code == INTERNAL.
}
```

The concrete-struct case has no equivalent test. `CardgroupUsecase` is a struct
literal type — there is no interface for the resolver-package mock to satisfy.
The resolver `unit` test for `CreateCardgroup` exercises the happy path and the
validation path through a real `CardgroupUsecase` instance wired to a stubbed
repository (see `cardgroup_resolvers_test.go`), and the nil-variant guard is
left **structurally unreachable in unit tests**:

```go
// cardgroup_resolvers_test.go — note on the nil-variant guard:
//
//   The resolver's CreateCardgroup method contains a defensive guard:
//
//     if outcome.Cardgroup == nil {
//         return nil, gqlerr.Internal(...)
//     }
//
//   CardgroupUsecase is a concrete struct (not an interface), so the resolver
//   cannot accept a mock implementation at the unit-test layer. The guard is
//   dead code: the usecase always sets outcome.Cardgroup on a nil-error path.
//   Its presence is a structural invariant, not a reachable branch.
```

The guard remains correct and load-bearing — a future producer that returns
a zero-value outcome would still trip the resolver into `INTERNAL` rather than
silently returning `nil, nil` — but the unit-test layer can no longer verify
it directly. The integration tests at `backend/cmd/server/main_test.go`
(`TestGraphQL_CreateCardgroup_*`) cover the surrounding behavior through a
real Postgres-backed server, but they cannot synthesize the nil-variant
state because the real usecase will never produce it on a nil-error path.

## How to apply

When promoting a mutation to an outcome union, choose the resolver field shape
deliberately:

- **Interface field** (preferred for new code): declare the usecase as an
  exported interface (`LastViewedCardgroupUsecase` is the precedent) and have
  the concrete implementation satisfy it. Resolver-package tests can mock the
  interface, exercise the nil-variant guard, and pin the structural invariant.

- **Concrete struct pointer** (legacy shape): the nil-variant guard is still
  required — see [`xor-invariant-outcome-struct.md`](xor-invariant-outcome-struct.md)
  for why — but accept that the unit-test coverage of the guard is structural
  only. Document the asymmetry inline at the resolver test (per the example
  above) so a future refactor does not silently remove the guard on the
  assumption that "there is no test for it". The nearest reachable proxy is
  a unit test that drives the **next** `gqlerr.Internal` path immediately
  following the nil-variant guard — typically an infrastructure precondition
  the resolver layer can synthesize without an interface mock (e.g. a nil tx
  runner on `NewSwipeUsecaseWithTx`, an unconfigured FSRS repo). Test names
  should reflect the true subject, not the structurally-unreachable guard:
  use `_InfrastructureError_ReturnsInternal` rather than `_NilVariant_ReturnsInternal`,
  and add a block comment explaining the substitution. Reference:
  `backend/graph/resolver/swipe_resolver_test.go` —
  `TestResolver_HandleSwipe_InfrastructureError_ReturnsInternal` builds a
  real `SwipeUsecase` with `tx = nil` and asserts the resolver returns
  `INTERNAL` via `gqlerr.FromUsecaseError`; the comment above the test
  records that the nil-variant guard at the resolver
  (`if outcome.Swipe == nil`) is structurally unreachable through the real
  usecase and the infrastructure path is the nearest reachable proxy for
  the INTERNAL mapping.

Migrating an existing concrete-struct field to an interface is a separate
refactor: extract the usecase's exported method set into an interface
declaration in the same package, change the resolver field type, and update
every production call site that constructs a Resolver. Out of scope for an
outcome-union promotion PR — the 800-line ceiling in
[`.claude/rules/pr-sizing.md`](../../../.claude/rules/pr-sizing.md) excludes
unrelated shape changes.
For the step-by-step mechanics of that promotion — the interface + unexported struct template,
type assertions for private access in same-package tests, `WithTx` constructor refactoring,
and the pointer-to-interface anti-pattern — see
[`usecase-interface-promotion-pattern.md`](usecase-interface-promotion-pattern.md).

## References

- `backend/graph/resolver/resolver.go` — `CardgroupUC` (concrete) vs
  `LastViewedCardgroupUC` (interface).
- `backend/graph/resolver/last_viewed_cardgroup_resolver_test.go` —
  `TestSetLastViewedCardgroup_NilVariant_ReturnsInternal` exercises the guard
  via the interface mock.
- `backend/graph/resolver/cardgroup_resolvers_test.go` — block comment above
  the `CreateCardgroup resolver tests` section explains why the symmetric
  test does not exist for the concrete-struct case.
