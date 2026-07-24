# Variadic optional dep injection is an antipattern in constructors

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A resolver or usecase constructor that accepts a dependency as a variadic argument
(`dep ...*usecase.SomeDep`) looks convenient — callers that do not need the
dependency can simply omit it. But this silently leaves the field as `nil` in any
caller that forgets (or chooses not) to pass it, and the nil dereference is deferred
to the first request that exercises the code path rather than surfacing at boot.

```go
// WRONG — variadic silently admits nil
func NewResolver(
    user *usecase.UserUsecase,
    learnUC ...usecase.LearnUsecase, // ← caller can omit, leaving r.LearnUC == nil
) *Resolver {
    r := &Resolver{UserUC: user}
    if len(learnUC) > 0 {
        r.LearnUC = learnUC[0]
    }
    return r
}

// resolver later:
func (r *Resolver) LearnNextDueCards(ctx context.Context, cardgroupID string, limit int) ([]*model.Card, error) {
    return r.LearnUC.NextDueCards(ctx, cardgroupID, time.Now().UTC(), limit) // nil panic when LearnUC was omitted
}
```

The fix is to make the dependency a required positional parameter. Tests that do not
exercise `learnNextDueCards` pass `nil` explicitly, making the intent visible at the
call site rather than hiding it behind variadic omission:

```go
// CORRECT — required positional parameter; tests pass nil explicitly
func NewResolver(
    user *usecase.UserUsecase,
    // ... other deps ...
    learnUC usecase.LearnUsecase, // ← tests that skip this feature pass nil
) *Resolver {
    return &Resolver{
        UserUC:  user,
        // ...
        LearnUC: learnUC,
    }
}
```

The resolver then performs a nil guard before use, or the production wiring
guarantees a non-nil value — either way the failure is explicit:

```go
// cmd/server/main.go wires non-nil for every production path
resolvers := resolver.NewResolver(
    userUC, cardgroupUC, cardUC, swipeUC, authSvc,
    cardImportUC, adminUserUC, adminRoleUC, lastViewedCGUC,
    learnUC, // always non-nil in production
)
```

**The same rule applies to wiring functions, not just constructors.** A function
like `newRouter` that accepts dependencies to pass into middleware or handlers must
declare every required dep as a named positional parameter. Using a variadic last
argument for the final repo (e.g. `swipeRecordRepo ...repository.SwipeRecordRepository`)
produces the same nil-deref hazard at the first request that touches that path
rather than surfacing at the call site.

**Reference:** `backend/graph/resolver/resolver.go` — `NewResolver` accepts
`learnUC usecase.LearnUsecase` as a required positional parameter; the comment
"Tests may pass nil for unused dependencies; do not pass nil from production
wiring" documents the nil-explicit contract. `backend/cmd/server/main.go` —
`newRouter` accepts `swipeRecordRepo repository.SwipeRecordRepository` as a
required positional parameter. `backend/internal/loader/loader.go` — `New` and
`Middleware` accept `swipeRecordRepo repository.SwipeRecordRepository` as a
required positional parameter; tests that do not exercise the SwipeRecord
loader pass `nil` explicitly.

**Sister rule:** [`constructor-panics-for-non-empty-config.md`](constructor-panics-for-non-empty-config.md) — when a dependency is always required (no OFF branch), panic at construction rather than deferring the nil deref to runtime.
