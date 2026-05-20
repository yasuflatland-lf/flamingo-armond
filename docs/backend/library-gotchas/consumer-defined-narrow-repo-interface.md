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
    FindDueCardsForUser(ctx context.Context, userID, cardgroupID string, now time.Time, limit int) ([]domain.DueCard, error)
}

// CardgroupRepoForLearn is the minimal cardgroup-repository surface.
type CardgroupRepoForLearn interface {
    FindByID(ctx context.Context, id string) (*domain.Cardgroup, error)
}
```

The shared `CardRepository` in `backend/internal/repository/card.go` keeps its full
surface (`FindByID`, `FindByIDs`, `FindPageByCardgroup`, `FindDueCardsForUser`,
`FindDueCardsForUserTx`, `Create`, `Update`, `Delete`, …). The `LearnUsecase` sees only
the two methods it calls, so:

- Test stubs for `LearnUsecase` need only two methods, not the full fourteen-method
  interface.
- Adding a new method to `CardRepository` for another feature never ripples into
  `LearnUsecase` test stubs.
- Grepping `CardRepoForLearn` immediately shows the complete data-access footprint
  of `LearnUsecase`, which the shared interface cannot provide.

**Antipattern:** adding `FindDueCardsForUser` to the shared `CardRepository` interface and
passing the concrete repository everywhere. This works until the test setup for a
`CardUsecase` test must implement `FindDueCardsForUser` even though that test exercises only
card-create logic. Over time each new cross-cutting method increases stub boilerplate
for every existing test suite.

### Return-type widening ripples through every narrow interface

When the concrete repository method's return type changes — for example
`[]*domain.Card` widening to `[]domain.DueCard` to carry per-user state — every
narrow interface that names the method (`CardRepoForLearn`, `CardRepoForSwipe`,
resolver test mocks) must be updated in lockstep. The Go compiler catches the
mismatch at every site, but a parallel-agent migration that fans out per file
without first pinning the new signature upstream produces a temporary build
break across every consumer site. Pin the interface contract first (in one
agent), then propagate to consumers (in fanned-out agents). The mechanical-
migration discipline in [`.claude/rules/subagent-dispatch.md` § "Symbol moves
must be atomic"](../../../.claude/rules/subagent-dispatch.md#symbol-moves-must-be-atomic-----one-agent-owns-both-delete-and-add)
applies to interface-shape changes for the same reason — a type rename split
across agents leaves an unbuildable intermediate state.

**Reference:** `backend/internal/usecase/learn.go` — `CardRepoForLearn` and
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
