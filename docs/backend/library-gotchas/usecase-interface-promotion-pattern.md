# Usecase interface promotion pattern

When a usecase starts as a concrete struct (`type CardgroupUsecase struct`) and is later
promoted to an interface, four concerns arise: the mechanics of promotion, test access to
private methods, `WithTx`-style constructor refactoring, and the pointer-to-interface
anti-pattern that surfaces during the change.

## Three-part template

Every promoted usecase follows this template (using `CardgroupUsecase` as the example):

```go
// Declared interface — lists only the methods the resolver calls.
type CardgroupUsecase interface {
    Cardgroup(ctx context.Context, id string) (*domain.Cardgroup, error)
    Create(ctx context.Context, in CreateCardgroupInput) (CreateCardgroupOutcome, error)
    Update(ctx context.Context, id string, in UpdateCardgroupInput) (UpdateCardgroupOutcome, error)
    Delete(ctx context.Context, id string) error
    ListCardgroupsByOwnerConnection(ctx context.Context, in CardgroupConnectionInput) (*CardgroupConnectionOutput, error)
}

// Unexported concrete struct — holds narrow repository interfaces and logger.
type cardgroupUsecase struct {
    repo   CardgroupRepository
    logger *slog.Logger
}

// Constructor returns the interface, not a pointer to the struct.
func NewCardgroupUsecase(repo CardgroupRepository, logger *slog.Logger) CardgroupUsecase {
    if logger == nil {
        panic("usecase: cardgroup: logger is required")
    }
    return &cardgroupUsecase{repo: repo, logger: logger}
}
```

The invariants:
- The interface name is public (`CardgroupUsecase`); the struct is unexported (`cardgroupUsecase`).
- Private methods on the struct — `resolveCardgroupCursor` here, and the same shape elsewhere
  (`resolveCardCursor` on `cardUsecase`, `clampLimit` on `learnUsecase`) — are
  excluded from the interface. Go's visibility rules enforce this: unexported methods cannot
  appear on a named interface in another package, and same-package code does not need them on
  the interface.
- `NewCardgroupUsecase` panics on nil dependencies. The panic is in the constructor, not in
  the methods, because the invariant must be enforced before the struct can be used at all.
  See [`constructor-panics-for-non-empty-config.md`](constructor-panics-for-non-empty-config.md).
- `cmd/server` wires the concrete `*gormCardgroupRepository` to the `CardgroupRepository`
  parameter and calls `NewCardgroupUsecase(...)`. The returned `CardgroupUsecase` interface
  value is passed directly to `resolver.NewResolver(...)`. No cast is needed; the concrete
  struct satisfies the interface implicitly.

The `Resolver` struct field is the interface type without a pointer prefix:

```go
type Resolver struct {
    CardgroupUC usecase.CardgroupUsecase   // interface, no *
    // ...
}
```

`NewResolver` accepts the same interface type for each usecase parameter. Taking `*usecase.CardgroupUsecase`
(a pointer to an interface) is almost always a bug — see the anti-pattern section below.

## Type assertion for private access in same-package tests

Tests in `package usecase` are in the same package as the implementation. After promotion,
the constructor returns `CardgroupUsecase` (an interface), not `*cardgroupUsecase`. But
test code sometimes needs to call a private method or set a private field directly — for
example, to unit-test `resolveCardgroupCursor` in isolation.

The correct pattern is a type assertion back to the concrete struct:

```go
// In cardgroup_test.go (package usecase):
uc := NewCardgroupUsecase(repo, slog.Default())
ordering := PageOrdering{OrderBy: string(orderBy), Direction: string(dir)}
got, err := uc.(*cardgroupUsecase).resolveCardgroupCursor(ctx, after, ownerID, orderBy, ordering, "after")
```

This is valid because:
1. The test is in `package usecase` — the same package as `cardgroupUsecase`. Go allows
   access to unexported fields and methods from within the same package, regardless of
   whether the access path goes through an interface value.
2. `NewCardgroupUsecase` always returns `*cardgroupUsecase` wrapped in the interface. The
   assertion will never panic in practice, but a failing assertion would surface as a test
   panic rather than a compile error.

The same pattern applies to field mutation in tests:

```go
// In swipe_error_test.go (package usecase):
uc := NewSwipeUsecaseWithTx(...)
uc.(*swipeUsecase).applyRating = func(...) error { return infraErr }
```

Do not add a `concrete()` or `inner()` accessor to the interface just to give tests access
to private members. The type assertion within the same package is the idiomatic alternative.

## `WithTx` constructor after interface promotion

Some usecases have a `WithTx`-style constructor that accepts a transaction runner so tests
can inject a controllable tx. Before promotion, the pattern was:

```go
// Pre-promotion: uc is *SwipeUsecase, so field assignment works.
func NewSwipeUsecaseWithTx(..., tx txRunner, ...) *SwipeUsecase {
    uc := NewSwipeUsecase(nil, ...)   // nil db — no real tx wired
    uc.tx = tx                        // direct field assignment
    return uc
}
```

After promotion `NewSwipeUsecase` returns `SwipeUsecase` (interface). Direct field
assignment on an interface value is not valid. Restore the pattern with a type assertion:

```go
// Post-promotion: assert to concrete struct, set field, return interface.
func NewSwipeUsecaseWithTx(..., tx txRunner, ...) SwipeUsecase {
    uc := NewSwipeUsecase(nil, ...).(*swipeUsecase)  // type-assert to get concrete ptr
    uc.tx = tx
    return uc
}
```

This works because the type assertion is in `package usecase` (same package as the
private struct). Production code outside the package cannot replicate this — which is
the correct boundary.

The alternative — duplicating the full constructor body in `NewSwipeUsecaseWithTx` — is
tempting but costly: any future change to the production constructor's default initialization
logic (e.g., changing `ordering` or `randSource` defaults) must be mirrored in the `WithTx`
variant, and the drift is invisible until a test relies on the differing behavior.

## Pointer-to-interface anti-pattern

`*usecase.CardgroupUsecase` is a pointer to an interface. This is almost always a bug.

An interface value in Go is already a two-word fat pointer (type descriptor + data pointer).
Taking a pointer to it adds a third layer of indirection. The practical consequences:

- An interface field in the `Resolver` struct declared as `*usecase.CardgroupUsecase` will
  not satisfy the `usecase.CardgroupUsecase` interface at assignment sites — the two types
  are different and the compiler rejects the assignment.
- A test that constructs `Resolver{CardgroupUC: &cgUC}` where `cgUC` is already of type
  `usecase.CardgroupUsecase` will fail with a "pointer to interface, not interface" error.
- Calling methods through `*usecase.CardgroupUsecase` requires double-dereference
  (`(*r.CardgroupUC).Create(...)`) and is not idiomatic.

The rule: a struct field whose type is a usecase interface must be declared without a `*`
prefix. The field holds the interface value (two words), not a pointer to the interface.

```go
// Correct:
CardgroupUC usecase.CardgroupUsecase

// Wrong — pointer to interface, not interface:
CardgroupUC *usecase.CardgroupUsecase
```

See [`resolver-usecase-interface-vs-concrete-testability.md`](resolver-usecase-interface-vs-concrete-testability.md)
for the testability rationale behind using interface fields in `Resolver`.
