# Mock snapshot at call time for "X before Y" ordering assertions

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a usecase mutates an aggregate in place (step X) and then calls a
repository method (step Y), a test must prove that X happened before Y — not
just that both happened. Reading the aggregate's field value through the
captured pointer **after** the usecase returns does not prove this: the pointer
reflects whatever state the aggregate ended up in, regardless of when the
mutation occurred.

## Symptom

A mock repository stores the pointer returned by `FindByID`:

```go
type mockCardgroupRepository struct {
    lastFoundCardgroup *domain.Cardgroup // captured in FindByID
    // ...
}

func (m *mockCardgroupRepository) FindByID(
    ctx context.Context, id string,
) (*domain.Cardgroup, error) {
    m.lastFoundCardgroup = m.findResult
    return m.findResult, m.findErr
}
```

The test then asserts the aggregate's `Name` field after the usecase returns:

```go
// WRONG — always passes regardless of where Rename was called
require.Equal(t, "New", string(repo.lastFoundCardgroup.Name))
```

This assertion passes even if `Rename` were called after `repo.Update`, or
removed entirely and the name were set by a direct field assignment anywhere
in the call chain.

## Why it passes incorrectly

`lastFoundCardgroup` is a pointer alias for `existing` inside the usecase.
When the usecase calls `existing.Rename(name)`, it mutates the struct through
that pointer. Reading `lastFoundCardgroup.Name` at test time always returns the
final state of the struct — there is no record of what the field held at the
moment `repo.Update` was invoked. Go pointer aliasing means the mock and the
usecase share the same memory location; the mock has no independent snapshot.

## Fix

Capture the field **value** (not the pointer) inside the mock's `Update` method
at the moment it is called:

```go
type mockCardgroupRepository struct {
    lastFoundCardgroup *domain.Cardgroup
    nameAtUpdateCall   domain.CardgroupName // snapshot taken inside Update
    // ...
}

func (m *mockCardgroupRepository) Update(
    ctx context.Context,
    id string,
    patch repository.CardgroupUpdate,
) (*domain.Cardgroup, error) {
    if m.lastFoundCardgroup != nil {
        m.nameAtUpdateCall = m.lastFoundCardgroup.Name // value copy, not pointer
    }
    return m.updateResult, m.updateErr
}
```

The assertion changes to:

```go
// RIGHT — fails if Rename is moved after repo.Update OR removed
require.Equal(t, domain.CardgroupName("New"), repo.nameAtUpdateCall)
```

Now the test pins the exact value of `Name` at the moment `repo.Update` was
invoked, proving that `Rename` (which sets `Name`) was called before `Update`.

## Caveats

Snapshot a copy or a plain value type, not another pointer to the same struct;
reset state reliably by constructing a fresh mock instance per test case rather
than relying on zero-value fields between table-driven sub-tests.

## References

- `backend/internal/usecase/cardgroup_test.go` — `mockCardgroupRepository.nameAtUpdateCall` field
  and `TestCardgroupUsecase_Update_NameChange_Success` for the aggregate-rename ordering test.
