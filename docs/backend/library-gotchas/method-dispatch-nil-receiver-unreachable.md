# Method dispatch on a nil pointer panics — `u == nil` guards in methods are unreachable

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

In Go, calling `u.Foo()` when `u` is nil panics inside the method dispatcher *before* a single line of `Foo`'s body executes. So an `if u == nil { ... }` guard at the top of a method body is unreachable in normal usage and only signals false confidence to readers — they will assume a nil receiver is gracefully handled when in fact a panic has already happened.

The guard is **also** unhelpful as a defensive layer because the only way to reach a method with `u == nil` would be to invoke it via a nil-typed function value or reflection, neither of which is a normal call site.

**What to keep:** field-level nil checks like `if u.fetcher == nil || u.cardgroupRepo == nil { ... }`. These remain meaningful — a method receiver may be non-nil while one of its dependency fields is nil due to a misconfigured constructor.

**Why:** removing the dead `u == nil` branch makes the method's preconditions accurate. A future reader who sees `if u.fetcher == nil` understands the constraint is on construction, not on receiver validity.

**Reference:** `NotionSyncUsecase.Sync` originally had `if u == nil || u.fetcher == nil || ...`; the `u == nil` term was removed because method dispatch had already panicked before reaching it.
