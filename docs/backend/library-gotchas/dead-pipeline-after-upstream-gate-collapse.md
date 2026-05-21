# Dead helper pipeline after an upstream gate — collapse to single wrap

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a usecase method gates an input through a parser or validator before calling
an inner helper, and the inner helper's only error path is the very case the gate
already rejected, the classification pipeline inside the inner `if` block is dead
code. Collapse it to a single `eris.Wrap`.

## Context

A usecase method often validates user input early — parsing it through a VO
constructor — and short-circuits before the business operation runs. If the inner
helper it calls afterwards has a single error path that mirrors the rejected case,
no error can arrive at that helper in production. Any structured pipeline
(translate → lift → fallthrough wrap) inside the inner block is unreachable.

## The shape that should be collapsed

```go
// AVOID: the inner pipeline is dead.
// ParseCardgroupName already rejects empty / too-long inputs.
// Rename's only error path is name == "" → ErrCardgroupNameRequired.
// That case can never reach Rename in production.

name, err := domain.ParseCardgroupName(input.Name, ...)
if err != nil {
    liftValidationErr(translateCardgroupNameErr(err)) // live — gate runs here
    ...
}

// Only reachable with a valid, non-empty name.
if renameErr := existing.Rename(name); renameErr != nil {
    info, err := liftValidationErr(translateCardgroupNameErr(renameErr))
    if err != nil {
        return UpdateCardgroupOutcome{}, err          // dead
    }
    if info != nil {
        return UpdateCardgroupOutcome{Validation: info}, nil  // dead
    }
    return UpdateCardgroupOutcome{}, eris.Wrap(renameErr, "usecase: cardgroup: rename")
}
```

The `info != nil` branch looks like forward-compatibility scaffolding for a
hypothetical future validation variant from `Rename`. It is not — it is unreachable
today and will remain unreachable as long as `Rename`'s contract stays the same.

## Why collapse

- **Zero test coverage on unreachable branches.** No test can drive `renameErr != nil`
  through production code paths because the gate above guarantees `name` is valid.
  Coverage tools report the lines as untested; the branches carry no invariant to
  protect.
- **A future reader cannot distinguish forward-compat scaffolding from live error
  handling.** The pipeline implies "Rename may return a validation error some day".
  That is misleading — the helper's contract is already fixed. The reader wastes time
  auditing a branch that does nothing.
- **The upstream gate is the contract; re-running classification implies the gate is
  unreliable.** Duplicating the translate-and-lift logic downstream signals that the
  caller does not trust the earlier guard. If the guard is correct, the inner copy is
  noise. If the guard is wrong, the inner copy masks the defect rather than exposing
  it.

## The collapsed form

```go
// PREFER: single wrap for defense-in-depth.
// Reachable only if a caller bypasses ParseCardgroupName
// via a direct CardgroupName("") cast — a caller bug, not a validation path.
if err := existing.Rename(name); err != nil {
    return UpdateCardgroupOutcome{}, eris.Wrap(err, "usecase: cardgroup: rename")
}
```

The single wrap preserves defense-in-depth (an unexpected error surfaces with a
canonical prefix rather than panicking) without implying that validation branching
is live.

## When NOT to collapse

If the inner helper has **multiple error return shapes** and the upstream gate only
eliminates one of them, keep the pipeline for the remaining paths. For example, if
`Rename` could also return a "name already taken" uniqueness error that the gate
does not check, the classification for that arm is live and must stay.

The collapse applies only when the gate provably eliminates **every** error path the
helper can return in production.

## Relationship to "Dead context-done branch in pass-through helper"

Same principle — dead branch → collapse — different trigger: the context-done
variant triggers on a redundant `isContextDone` check where both arms return the
same value; this variant triggers on an upstream gate that eliminates the only error
the downstream helper can produce.

## References

- `backend/internal/usecase/cardgroup.go` — post-collapse `Rename` call site uses
  the single-wrap form shown above.
- [`.claude/rules/go-library-gotchas.md`](../../../.claude/rules/go-library-gotchas.md) —
  "Dead context-done branch in pass-through helper" entry for the sibling pattern.
