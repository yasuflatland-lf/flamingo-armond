# Go `map` is a reference type — copy in the constructor when accepting one

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A constructor that stashes a caller-supplied `map` directly (`p.emails = emails`) leaves the invariant under the caller's control: any later `delete(emails, k)` or `emails[k] = struct{}{}` mutates the constructed object's internal state without going through any of its methods. This is silent and almost impossible to track down because the receiver has no API surface that names the violation. `slice` has the same property; the fix shape is the same.

```go
emailsCopy := make(map[string]struct{}, len(emails))
for k := range emails { emailsCopy[k] = struct{}{} }
return &SuperUserPromoter{ emails: emailsCopy, /* ... */ }
```

The copy is `O(n)` once at construction and the receiver's invariants are now tamper-proof. Apply to any constructor whose stored field is a reference type (`map`, `slice`, `chan`) and whose correctness depends on the contents being stable.
