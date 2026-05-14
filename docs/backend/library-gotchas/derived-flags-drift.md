# Derived flags drift; read the source of truth instead

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A field that mirrors a property already derivable from another field on the same struct introduces two states the type system does not enforce to agree. Any future constructor variant, copy, or mutation path that updates one and forgets the other yields a struct whose hot-path branch disagrees with its own data.

```go
// AVOID: `enabled` is fully determined by `len(items)`, but the compiler does
// not enforce that the two stay in sync across all constructors.
type Filter struct {
    items   map[string]struct{}
    enabled bool // derived from len(items) > 0
}

// PREFER: compute on read; impossible to drift.
type Filter struct {
    items map[string]struct{}
}
func (f *Filter) Enabled() bool { return len(f.items) > 0 }
```

Prefer recomputation unless profiling shows the read is hot enough to matter. For middleware factories or constructors that run once per process (not per request), the cost is rounding-error and not worth the drift risk.
