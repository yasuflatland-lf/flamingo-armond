# Int-typed domain enums need `IsValid()` on DB reconstitution

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Go lets you cast any integer to a named type without a bounds check:

```go
type FSRSPhase int

state := FSRSPhase(row.State) // compiles even when row.State = 99
```

When the raw integer comes from a DB column, any out-of-range value is accepted silently. A migration bug, a manual SQL edit, or a future schema change can store a value that no switch case handles, and the domain object carrying it becomes invalid without any error being raised.

Add an `IsValid() bool` method to every int-typed domain enum, and call it at every reconstitution site that casts a raw DB integer to that type:

```go
// domain/fsrs_state.go
type FSRSPhase int

const (
    FSRSPhaseNew FSRSPhase = iota
    FSRSPhaseLearning
    FSRSPhaseReview
    FSRSPhaseRelearning
)

func (p FSRSPhase) IsValid() bool {
    return p >= FSRSPhaseNew && p <= FSRSPhaseRelearning
}

// repository/user_card_fsrs.go — reconstitution site
func userCardFSRSToDomain(row gormUserCardFSRS) (*domain.UserCardFSRS, error) {
    state := domain.FSRSPhase(row.State)
    if !state.IsValid() {
        return nil, eris.Errorf("repository: invalid FSRSPhase value %d for card %s", row.State, row.CardID)
    }
    // ... build the domain object
}
```

**Where to call `IsValid()`:** anywhere that casts a raw `int` (or `int32`, `int64`, etc.) to a domain enum type. The most common site is the repository's row-to-domain conversion function. A reconstitution that skips the check propagates invalid state silently through every layer above it — usecase, resolver, API response — until a switch statement falls through to a default branch or a panic.

**Testing:** add a table-driven repository test that inserts a row with an out-of-range state value directly via SQL and asserts that the repository's reconstitution returns a non-nil error wrapping the out-of-range integer.

Reference: `backend/internal/domain/fsrs_state.go` — `FSRSPhase.IsValid()`. `backend/internal/repository/user_card_fsrs.go` — `userCardFSRSToDomain` guards the cast with `IsValid()` and returns an `eris.Errorf` on failure.
