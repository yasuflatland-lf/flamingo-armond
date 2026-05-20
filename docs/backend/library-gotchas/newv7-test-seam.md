# `newV7` indirection seam for rare-failure crypto helpers

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

Helpers wrapping `uuid.NewV7`, `crypto/rand.Read`, `os.UserHomeDir`, and
similar standard-library functions fail only in degraded-environment conditions
(no `crypto/rand` source, missing user-home, etc.) that cannot be triggered
from a normal test. When acceptance criteria require proving the `eris.Wrap`
prefix is present in the error chain on the failure path, an injection seam is
the only mechanism that makes the assertion possible. Without one, a future
refactor that drops the `eris.Wrap` silently degrades the chain shape with no
test regression.

## The pattern

Introduce a package-private `var` that holds the real function. Tests swap it
via `t.Cleanup`; production code is unchanged.

```go
// newV7 is the indirection seam for tests; production code calls uuid.NewV7.
var newV7 = uuid.NewV7

func NewID() (string, error) {
    id, err := newV7()
    if err != nil {
        return "", eris.Wrap(err, "domain: new uuid v7")
    }
    return id.String(), nil
}
```

Test:

```go
// Swaps the package-level seam to simulate crypto/rand unavailability;
// must not run in parallel because it mutates a package-level variable.
t.Run("failure path: wrap prefix present in error chain", func(t *testing.T) {
    orig := newV7
    newV7 = func() (uuid.UUID, error) {
        return uuid.UUID{}, errors.New("rand: unavailable")
    }
    t.Cleanup(func() { newV7 = orig })

    _, err := NewID()
    require.Error(t, err)
    require.True(t, strings.Contains(err.Error(), "domain: new uuid v7"),
        "error chain must contain wrap prefix %q, got: %v", "domain: new uuid v7", err)
})
```

## Constraints

**No `t.Parallel()` on the swap sub-test.** The sub-test mutates a package-level
variable; running it concurrently with any other test that calls `NewID` produces
a data race.

**Keep the seam unexported.** `newV7`, `newRand`, `newHomeDir` — the lowercase
form ensures the seam never leaks as public API. An exported hook
(`SetNewV7Func(...)`) converts a test aid into a supported public surface.

**One seam per fault domain.** Do not share a single seam across helpers that
fail for independent reasons. Coupling unrelated helpers behind one var creates
surprising test interactions when the stub is active.

## Reference

`backend/internal/domain/ids.go` — `var newV7` seam and `NewID` helper;
`backend/internal/domain/ids_test.go` — swap pattern and chain-shape assertion.

The originating spec (issue #199, Item 4) required asserting `"domain: new uuid v7"`
in the error chain. The seam made the assertion mechanically sound rather than
contingent on an unreproducible environment failure.
