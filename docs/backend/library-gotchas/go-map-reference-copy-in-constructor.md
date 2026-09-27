# Go `map` is a reference type — copy in the constructor when accepting one

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A constructor that stashes a caller-supplied `map` directly (`g.allowed = allowed`) leaves the invariant under the caller's control: any later `delete(emails, k)` or `emails[k] = struct{}{}` mutates the constructed object's internal state without going through any of its methods. This is silent and almost impossible to track down because the receiver has no API surface that names the violation. `slice` has the same property; the fix shape is the same.

```go
allowedCopy := make(map[string]struct{}, len(allowed))
for k := range allowed { allowedCopy[k] = struct{}{} }
return &allowlistGate{ allowed: allowedCopy }
```

The copy is `O(n)` once at construction and the receiver's invariants are now tamper-proof. Apply to any constructor whose stored field is a reference type (`map`, `slice`, `chan`) and whose correctness depends on the contents being stable.
