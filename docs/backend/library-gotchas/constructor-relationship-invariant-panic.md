# Constructor panics must cover argument-relationship invariants, not just nil checks

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Individual nil checks in a constructor catch the obvious missing-dependency case, but
some constructors also carry invariants *between* arguments. When those invariants
are violated the struct is constructed without error and misbehaves at runtime —
often non-deterministically when the bad configuration is exercised only under load.

Panic at construction for relationship invariants for the same reason nil checks panic:
the violation is an operator-visible programming error, not a runtime input, and
`run()` has not yet started the HTTP server when it fires, so the panic crashes the
process at boot rather than degrading requests.

```go
func NewLearnUsecase(
    cardRepo      CardRepoForLearn,
    cardgroupRepo CardgroupRepoForLearn,
    ordering      *service.OrderingPolicy,
    randSource    func() *rand.Rand,
    defaultLimit, maxLimit int,
) *LearnUsecase {
    // nil guards first
    if cardRepo == nil {
        panic("LearnUsecase: cardRepo must not be nil")
    }
    if cardgroupRepo == nil {
        panic("LearnUsecase: cardgroupRepo must not be nil")
    }

    // relationship invariant: defaultLimit may never exceed maxLimit
    if defaultLimit > maxLimit {
        panic(fmt.Sprintf(
            "LearnUsecase: defaultLimit (%d) must not exceed maxLimit (%d)",
            defaultLimit, maxLimit,
        ))
    }

    return &LearnUsecase{
        cardRepo:      cardRepo,
        cardgroupRepo: cardgroupRepo,
        ordering:      ordering,
        randSource:    randSource,
        defaultLimit:  defaultLimit,
        maxLimit:      maxLimit,
    }
}
```

Without the relationship panic, passing `defaultLimit=50, maxLimit=20` constructs a
`LearnUsecase` whose `clampLimit` helper will always clamp to 20, silently ignoring
the misconfigured default. The bug surfaces only when a caller relies on the default
and receives fewer cards than expected — with no error in the log.

**Scope:** relationship-invariant panics apply to config-shaped constructors. If every
call site must always satisfy the invariant (it is not configuration-dependent), a
simpler alternative is to hard-code the constants inside the package and expose no
knob at all. Introduce the knob only when it genuinely varies across environments
or test cases.

**Reference:** `backend/internal/usecase/learn.go` — `NewLearnUsecase` panics on both
nil dependencies and the `defaultLimit > maxLimit` relationship.

**Sister rule:** [`constructor-panics-for-non-empty-config.md`](constructor-panics-for-non-empty-config.md) — the foundational rule for constructor panics on nil/empty config.
