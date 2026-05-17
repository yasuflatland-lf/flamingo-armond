# Bool flag vs two-function split: ubiquitous language signals

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

A bool parameter that selects between two distinct error outcomes is a code smell:
the function name no longer expresses which operation the caller is performing.
DDD's ubiquitous language principle says different operations deserve different names.

## The antipattern

```go
// AVOID: the bool encodes two separate business operations.
// Call sites reveal nothing about intent.
func authorizeCardgroup(
    ctx context.Context,
    repo CardgroupOwnershipFinder,
    id, userID string,
    missingAsBadInput bool,
) error {
    cg, err := repo.FindByID(ctx, id)
    if err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            if missingAsBadInput {
                return ucerr.NewValidationError("cardgroupId", "cardgroup not found")
            }
            return ucerr.ErrUnauthenticated
        }
        return eris.Wrap(err, "usecase: authorize cardgroup")
    }
    if cg.UserID != userID {
        return ucerr.ErrUnauthenticated
    }
    return nil
}

// Call sites — intent is opaque:
authorizeCardgroup(ctx, repo, id, userID, true)
authorizeCardgroup(ctx, repo, id, userID, false)
```

## The fix: two functions with expressive names

```go
// PREFER: each function name expresses what it does on not-found.

// authorizeCardgroupOrBadInput is used when the caller received the
// cardgroup ID from user-supplied input (e.g. a mutation argument) and a
// missing ID is a validation error the client can correct.
func authorizeCardgroupOrBadInput(
    ctx context.Context,
    repo CardgroupOwnershipFinder,
    id, userID string,
) error {
    cg, err := repo.FindByID(ctx, id)
    if err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            return ucerr.NewValidationError("cardgroupId", "cardgroup not found")
        }
        return eris.Wrap(err, "usecase: authorize cardgroup or bad input")
    }
    if cg.UserID != userID {
        return ucerr.ErrUnauthenticated
    }
    return nil
}

// authorizeCardgroupOrUnauthenticated is used when the caller derives the
// cardgroup ID from a trusted path (e.g. an existing card's FK) and a
// missing ID means the session is stale or the resource was deleted.
func authorizeCardgroupOrUnauthenticated(
    ctx context.Context,
    repo CardgroupOwnershipFinder,
    id, userID string,
) error {
    cg, err := repo.FindByID(ctx, id)
    if err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            return ucerr.ErrUnauthenticated
        }
        return eris.Wrap(err, "usecase: authorize cardgroup or unauthenticated")
    }
    if cg.UserID != userID {
        return ucerr.ErrUnauthenticated
    }
    return nil
}

// Call sites — intent is explicit:
authorizeCardgroupOrBadInput(ctx, repo, id, userID)
authorizeCardgroupOrUnauthenticated(ctx, repo, id, userID)
```

## When the split is warranted

A bool flag is a signal for splitting when:

1. The two branches represent **semantically different operations** — here, "this is
   user-supplied input" vs. "this came from a trusted FK". The type system cannot
   enforce which path a caller is on, but the function name can.
2. The **error type emitted differs** between branches. A caller reading only the
   function name at a glance should be able to predict the error kind without
   inspecting the body.
3. There is no shared state setup that justifies a single entry point. If both
   branches share 30 lines of setup and differ in only 2, an internal helper with
   a small enum is acceptable; but for a 10-line function that branches entirely
   on the flag, the split costs nothing.

## Bool is not always wrong

A bool flag is fine when it controls a non-semantic detail — for example,
`verbose bool` on a debug formatter, or `includeDeleted bool` on a read-only
query whose error path is identical in both cases. The smell is specifically
"different observable outcomes that callers must reason about differently".

## Analogous pattern in this codebase

`TrimAndDetect[T]` and `TrimAndDetectBackward[T]` encode direction via two
function names rather than a `reverse bool` flag. The separation makes the
pagination helper's directionality self-documenting at every call site. See
[`.claude/rules/pagination.md`](../../../.claude/rules/pagination.md) for the
`+1` fetch trick those helpers support.

## References

- [`.claude/rules/error-wrapping.md`](../../../.claude/rules/error-wrapping.md) — typed errors (`ucerr.*`) returned by usecase code.
- [`consumer-defined-narrow-repo-interface.md`](consumer-defined-narrow-repo-interface.md) — the `CardgroupOwnershipFinder` interface used in the examples above.
