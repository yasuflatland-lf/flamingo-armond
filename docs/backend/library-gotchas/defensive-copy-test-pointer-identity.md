# Defensive-copy tests need a FromPtr-copy check and a Ptr-identity check

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

## The trap

A pointer-wrapping value object such as `Bio` copies in two places: `BioFromPtr`
copies the caller's `*string` on the way in, and `Bio.Ptr()` hands out a fresh
`*string` on the way out. Each copy guards a different aliasing path, and no
single assertion covers both. A test that pins only one of them leaves the
other free to regress.

The easy mistake is to believe one assertion proves both, or to dismiss one as
useless. Both are needed.

## Assertion 1 — FromPtr copies its input

```go
s := "original"
b := BioFromPtr(&s)
s = "mutated"                          // writes the variable that &s points to
require.Equal(t, "original", *b.Ptr()) // fails if BioFromPtr kept &s
```

`s = "mutated"` is an assignment to the variable `s`, and `&s` is the address of
that variable. If `BioFromPtr` stored the caller's pointer verbatim, the `Bio`
would observe the new value and this assertion would fail. String immutability
is about the bytes a string header points at, not about the variable holding
the header, so it does not make this test vacuous.

## Assertion 2 — Ptr returns a fresh pointer on each call

```go
p1, p2 := b.Ptr(), b.Ptr()
require.NotSame(t, p1, p2, "each Ptr() call must allocate a fresh pointer")
require.Equal(t, *p1, *p2, "but the values must be equal")
```

`require.NotSame` compares the `*string` addresses. If `Ptr()` returned the
stored pointer, both calls would yield the same address and a caller could
write through it into the value object. The companion `require.Equal` confirms
the values still agree.

This assertion does **not** detect a `BioFromPtr` that kept the caller's
pointer: when `Ptr()` copies on every call, `p1` and `p2` differ even though the
stored pointer is still aliased to the caller. Only assertion 1 catches that.

## Why this happens

Each copy closes a different aliasing path:

- **Inbound** — the caller keeps the `*string` it passed to `FromPtr` and can
  assign through it later (`*p = "x"`, or `s = "x"` on the variable it
  addresses). The FromPtr copy prevents that from reaching the value object.
- **Outbound** — the caller receives a `*string` from `Ptr()` and can assign
  through it. The fresh copy per call prevents that from reaching the value
  object or other callers.

A defensive-copy contract is "nothing I store or return is aliased to a pointer
someone else holds". Test both directions.

## Reference

- `backend/internal/domain/text_vo_property_test.go` — `TestTrinaryText_Property_BioAndDescription` asserts `require.NotSame` on two `Ptr()` calls and the FromPtr-copy check for every accepted non-nil Bio and Description.
- `backend/internal/domain/bio.go` — `Bio.Ptr()` allocates a fresh `*string` on every call.
