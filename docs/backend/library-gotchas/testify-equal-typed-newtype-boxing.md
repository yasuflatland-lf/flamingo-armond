# testify `require.Equal` silently fails on typed string newtypes

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## The trap

When a struct field is a typed string newtype — `domain.UserID`, `domain.CardgroupID`,
`domain.CardText`, `domain.CardgroupName` — comparing it against a plain `string`
with `require.Equal` (or `assert.Equal`) **compiles without error but fails at
runtime**. testify boxes both arguments to `interface{}` and calls `ObjectsAreEqual`,
which compares dynamic types first. The dynamic type of `domain.UserID("x")` is
`domain.UserID`; the dynamic type of `"x"` is `string`. They are not equal, so
the assertion reports a mismatch even though the underlying character sequence
is identical.

```go
// Compiles, fails at runtime.
require.Equal(t, "cg-1", c.CardgroupID)      // want domain.CardgroupID, got string
require.Equal(t, ownerID, got.OwnerID)        // ownerID is string; got.OwnerID is domain.UserID
```

`go build` and `go vet` do NOT catch this. Both arguments satisfy `interface{}`, so
the call is type-correct. Only running the tests surfaces the failure.

## Two flavors — and why a literal-only grep misses the second

**Flavor 1 — string literal on the expected side:**

```go
require.Equal(t, "u-1", got.OwnerID)   // "u-1" is an untyped string constant
```

A grep for `require.Equal(t, "` catches this pattern.

**Flavor 2 — string variable on the expected side:**

```go
ownerID := "owner-abc"                           // plain string variable
require.Equal(t, ownerID, string(got.OwnerID))   // safe — explicit cast
// But without the cast:
require.Equal(t, ownerID, got.OwnerID)            // compiles, fails at runtime
```

A literal-grep misses Flavor 2 entirely. It only surfaces when the test suite is run.
This is the form that bit `backend/internal/repository/cardgroup_test.go` during
the issue #438 newtype migration: `ownerID` was a captured `string` variable from
the test fixture, and the assertion silently failed after `Cardgroup.OwnerID` was
changed from `string` to `domain.UserID`.

## The fixes

**Option A — wrap the expected side in the newtype:**

```go
require.Equal(t, domain.UserID("u-1"), got.OwnerID)
require.Equal(t, domain.CardgroupID("cg-1"), got.CardgroupID)
```

This is the canonical form inside `backend/internal/domain/` package tests, where
the types are in scope without the `domain.` qualifier:

```go
require.Equal(t, UserID("owner-001"), cg.OwnerID)    // domain package test
require.Equal(t, CardgroupID("cg-1"), c.CardgroupID) // domain package test
```

**Option B — cast the actual side to string:**

```go
require.Equal(t, ownerID, string(got.OwnerID))
```

Use this form when the expected value is already a plain `string` variable and
wrapping in the newtype constructor would require importing the `domain` package
just for the cast. The repository integration tests use this form consistently:

```go
require.Equal(t, ownerID, string(got.OwnerID))   // cardgroup_test.go lines 47, 258, 604
```

**Map keys and slice elements** need the same treatment. A `map[string]struct{}`
keyed by entity IDs must use `string(id)` at every lookup and insertion site:

```go
m[string(cg.ID)] = struct{}{}
if _, ok := m[string(cg.ID)]; ok { ... }
```

`reflect.DeepEqual` and any other `interface{}`-boxing equality function exhibit
the same type-mismatch behaviour.

## The harness: run tests, not just build/vet

After introducing a string newtype and migrating struct fields to it, running
`go build ./...` and `go vet ./...` is insufficient. Both complete successfully
because the call is type-correct at the Go level. The required gate is:

```bash
go test -v -race -covermode=atomic ./...
```

The test runner surfaces every `Not equal` assertion failure that build/vet
silently accepted. Make this the final step of any newtype migration — not an
optional check after the diff looks clean.

## Grep both arg orders, then run the suite

Two grep patterns cover both flavors of the trap. Run them against every
`_test.go` file touched by the migration:

```bash
# Flavor 1: string literal in the expected (first) position
grep -rn 'require\.Equal(t, "' backend/ --include='*_test.go'

# Flavor 2: typed field compared against a string variable — no grep can
# distinguish this from a correctly typed comparison; only the test suite can.
go test -v -race ./...
```

A literal grep finding zero hits does not mean the codebase is clean. The suite
run is required.

## Worked example — issue #438

During the DDD ID-typing cleanup that changed `Cardgroup.OwnerID` from `string`
to `domain.UserID` and `Cardgroup.ID` from `string` to `domain.CardgroupID`,
three test files required assertion fixes:

- **`backend/internal/domain/card_test.go`** (`TestNewCard`): the assertion for
  `c.CardgroupID` was corrected to use `CardgroupID("cg-1")` (Flavor 1 —
  string literal on the expected side).

- **`backend/internal/domain/cardgroup_test.go`** (`TestCardgroup_Rename`): the
  post-call assertions on `cg.ID` and `cg.OwnerID` were corrected to
  `CardgroupID("cg-id-001")` and `UserID("owner-001")`.

- **`backend/internal/repository/cardgroup_test.go`**: multiple assertions of the
  form `require.Equal(t, ownerID, got.OwnerID)` — where `ownerID` is a `string`
  variable captured from the test setup — were corrected to
  `require.Equal(t, ownerID, string(got.OwnerID))` (Flavor 2, Option B fix).
  Lines 47, 258, and 604 show the settled form. Additionally, cursor-page
  assertions that expected `domain.CardgroupID(sortedIDs[i])` on the actual side
  (lines 871–872) already used the newtype constructor correctly and passed.

The domain-package tests were caught by the test suite immediately; the
repository-test Flavor 2 failures only appeared after the integration test suite
ran against a real database. Neither `go build` nor `go vet` flagged any of them.
