# Consumer-defined narrow repository interface per usecase

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a new usecase needs one or two methods from a repository, the instinct is to add
those methods to the shared `CardRepository` interface everyone already uses. That
works, but it forces every mock and stub across the codebase to implement the new
methods — even tests that never exercise the new usecase.

Instead, declare a local interface in the usecase file that lists only what *this*
usecase calls. Go's structural typing satisfies the interface automatically without
any change to the concrete repository:

```go
// backend/internal/usecase/learn.go

// CardRepoForLearn is the minimal card-repository surface that LearnUsecase needs.
// The concrete *repository.cardRepo satisfies it automatically.
type CardRepoForLearn interface {
    FindDueCards(ctx context.Context, cardgroupID string, now time.Time, limit int) ([]*domain.Card, error)
}

// CardgroupRepoForLearn is the minimal cardgroup-repository surface.
type CardgroupRepoForLearn interface {
    FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}
```

The shared `CardRepository` in `backend/internal/repository/card.go` keeps its full
surface (`FindByID`, `FindByIDs`, `FindPageByCardgroup`, `FindDueCards`,
`FindDueCardsTx`, `Create`, `Update`, `Delete`, …). The `LearnUsecase` sees only
the two methods it calls, so:

- Test stubs for `LearnUsecase` need only two methods, not the full fourteen-method
  interface.
- Adding a new method to `CardRepository` for another feature never ripples into
  `LearnUsecase` test stubs.
- Grepping `CardRepoForLearn` immediately shows the complete data-access footprint
  of `LearnUsecase`, which the shared interface cannot provide.

**Antipattern:** adding `FindDueCards` to the shared `CardRepository` interface and
passing the concrete repository everywhere. This works until the test setup for a
`CardUsecase` test must implement `FindDueCards` even though that test exercises only
card-create logic. Over time each new cross-cutting method increases stub boilerplate
for every existing test suite.

**Reference:** `backend/internal/usecase/learn.go:21–28` — `CardRepoForLearn` and
`CardgroupRepoForLearn` as local interfaces satisfied by the concrete repository.
The same pattern appears in `internal/usecase/admin_role.go` as `adminRoleRepoForCRUD`
(documented in `docs/backend-graphql.md` § "Admin usecase split").

## Shared helpers can declare their own narrow interface

The consumer-defined narrow interface pattern also applies to **package-internal
helper functions**, not just usecase struct fields. When a helper accepts only a
subset of a repository's surface, declare the interface at the helper's site,
not at the consumer struct.

Worked example from `backend/internal/usecase/ownership.go`:

```go
// CardgroupOwnershipFinder is the narrow repo surface ownership checks need.
type CardgroupOwnershipFinder interface {
    FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}

func authorizeCardgroupOrBadInput(
    ctx context.Context,
    repo CardgroupOwnershipFinder, // narrow, not full repo
    id, userID string,
) error { ... }
```

The helper is package-internal but the interface keeps it independently
mockable, and prevents accidental expansion of the helper's repository
dependency footprint. If a future helper needs additional methods, it declares
its own interface rather than widening this one. The ISP cost — one extra
interface declaration — is paid once at the helper site; the readability and
test-isolation payoff repeats at every call site and test.
