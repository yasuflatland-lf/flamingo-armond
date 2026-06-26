# Call-count error injection on a fake to cover the Nth call of a twice-called repo method

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a usecase calls the **same repository method more than once in one operation**, a
fake that injects an error through a single error field can only exercise the *first*
call — the later call's error branch stays untested, and a missing guard there is a
silent gap.

The canonical case is an operation that gates ownership and then re-reads the same
aggregate: `MergeMasterIntoCardgroup` calls `u.userCG.FindByID` twice — once inside the
pre-tx `authorizeCardgroupOrBadInput` gate, and again after the transaction commits to
build the success payload. A fake with a plain `findByIDErr error` field fails the
*ownership gate* (call 1) and never reaches the post-tx read (call 2), so the post-tx
`isContextDone` guard and the `eris.Wrap(..., "...: find destination")` branch are
unreachable by the test — exactly the branch a reviewer flagged as unguarded.

**Fix:** give the fake a 1-based call counter and an "error on call N" field. The zero
value (`0`) never injects, so every existing test that constructs the fake without the
field is unaffected.

```go
type fakeUserCG struct {
	byID              map[string]*domain.Cardgroup
	findByIDCalls     int
	findByIDErr       error
	findByIDErrOnCall int // 1-based; 0 = never inject (default for all existing tests)
}

func (f *fakeUserCG) FindByID(_ context.Context, id string) (*domain.Cardgroup, error) {
	f.findByIDCalls++
	if f.findByIDErrOnCall != 0 && f.findByIDCalls == f.findByIDErrOnCall {
		return nil, f.findByIDErr
	}
	if cg, ok := f.byID[id]; ok {
		return cg, nil
	}
	return nil, repository.ErrNotFound
}
```

Then the test injects on the call that reaches the branch under test (`findByIDErrOnCall: 2`
= ownership gate succeeds on call 1, post-tx read fails on call 2):

```go
userCG := &fakeUserCG{
	byID:              map[string]*domain.Cardgroup{destID: destCG},
	findByIDErr:       context.Canceled, // or errors.New("db down")
	findByIDErrOnCall: 2,
}
```

Two follow-on assertions differ by error kind, because the boundary's handling differs:

- **Context cancel on the post-tx read** is returned **bare** (the `isContextDone` guard
  does `return nil, err` without wrapping), so assert *both* pointer identity
  (`err != context.Canceled → t.Fatal`) **and** `assertCancelled`. This is unlike a
  cancel raised **inside** the tx closure, which is `eris.Wrap`-ed before the outer
  `isContextDone` returns it — there only `assertCancelled` (an `errors.Is` walk) holds.
- **An infra error on the post-tx read** is wrapped, so assert the chain prefix:
  `assertInternalChain(t, err, "usecase: master deck: merge master into cardgroup: find destination")`.

Related: [`test-stub-fatal-on-exhausted-fixture.md`](test-stub-fatal-on-exhausted-fixture.md)
(queue-style stubs that pop per call — a different multi-call shape) and
[`pin-unwrapped-context-error-with-identity-check.md`](../error-wrapping/pin-unwrapped-context-error-with-identity-check.md)
(why the bare-vs-wrapped context distinction is asserted with pointer identity).
