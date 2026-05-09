# Derived flags drift; read the source of truth instead

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A `passthrough bool` field on a struct that is "kept in sync with `len(emails) == 0`" introduces two states that the type system does not enforce to agree. Any future constructor variant, copy, or mutation path that sets one and forgets the other produces a struct whose hot-path branch disagrees with its data. The fix is to delete the cache and read the source of truth at the decision point:

```go
// AVOID: derived flag duplicates state already in p.emails.
type SuperUserPromoter struct { emails map[string]struct{}; passthrough bool /* derived */ }

// PREFER: compute on read; impossible to drift.
if len(p.emails) == 0 { /* pass-through branch */ }
```

The rule generalises to any field that is fully determined by another field on the same struct: prefer recomputation unless profiling shows the read is hot enough to matter. For an Echo middleware factory `Middleware()` that runs once per process (not per request), the cost is rounding-error.
