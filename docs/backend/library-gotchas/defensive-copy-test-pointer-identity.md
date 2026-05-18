# Defensive-copy tests assert pointer identity, not variable rebind

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## The trap

A naive test for a defensive-copy accessor reaches for variable mutation to
prove the copy survived:

```go
// WRONG — proves nothing about defensive copying.
func TestBio_DefensiveCopy_WRONG(t *testing.T) {
    s := "original"
    b := BioFromPtr(&s)
    s = "mutated"                              // <- rebinds local, doesn't mutate the original string
    require.Equal(t, "original", *b.Ptr())     // passes whether or not Bio defensively copies
}
```

This test passes regardless of whether `BioFromPtr` defensively copies, because
Go strings are immutable. `s = "mutated"` does not write into the existing
string header that `&s` once pointed at; it allocates a new string and rebinds
the local `s` to that new value. The original string the `Bio` captured was
never mutable, so a non-defensive `Bio` would still report `"original"`.

A defensive-copy invariant exists for *external aliasing*: another caller
holding the same `*string` could swap out the pointer's referent. But Go
strings are immutable, so swapping is the only way to "mutate" them — and the
test above does not swap, it rebinds.

## The fix — assert pointer identity

The defensive-copy property to test is: each `Ptr()` call hands the caller a
fresh `*string`, so two consecutive calls return non-identical pointers:

```go
// CORRECT — proves each call allocates a fresh pointer.
t.Run("Ptr returns a fresh pointer on each call", func(t *testing.T) {
    s := "original"
    b := BioFromPtr(&s)
    p1, p2 := b.Ptr(), b.Ptr()
    require.NotSame(t, p1, p2, "each Ptr() call must allocate a fresh pointer")
    require.Equal(t, *p1, *p2, "but the values must be equal")
})
```

`require.NotSame` compares the `*string` addresses. Two fresh allocations have
different addresses; a shared pointer would return the same address twice and
fail the assertion. The companion `require.Equal(t, *p1, *p2)` confirms the
values still agree, so the test does not accidentally pass on a broken
implementation that returns a fresh pointer pointing at unrelated data.

The same assertion shape catches the failure mode the rebind test misses:
if `BioFromPtr` were changed to store the caller's pointer verbatim (no
internal copy), `b.Ptr()` would return that exact pointer on every call, and
`require.NotSame(p1, p2)` would fail.

## Why this happens

The trap is that the word "mutation" is overloaded in Go:

- **Local variable rebind** (`s = "x"`) — changes which value the local
  identifier refers to. Does not write into any pre-existing memory.
- **Pointee mutation** through an exclusive pointer (`*p = "x"`) — also legal
  for strings, but again allocates a new string and rebinds the location `p`
  refers to.
- **Slice-element or struct-field mutation** through a shared pointer — the
  case that defensive copies actually defend against, but not what string
  immutability allows.

A defensive-copy accessor's contract is "the value I return is not aliased
to the value I stored". The right test exercises the aliasing axis (pointer
identity), not the rebind axis.

## Reference

- `backend/internal/domain/bio_test.go` — `TestBioFromPtr` / `"Ptr returns a fresh pointer on each call"`.
- `backend/internal/domain/bio.go` — `Bio.Ptr()` allocates a fresh `*string` on every call.
