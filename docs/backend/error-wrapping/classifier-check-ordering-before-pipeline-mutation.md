# Classifier check must run before any pipeline step that appends to the classified slice

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## What

When a function (a) classifies a slice of errors by inspecting every element, and (b) a subsequent pipeline step appends new errors to the same slice, the classification MUST happen before the append. Running the classifier after the append causes it to see elements it was not meant to classify, silently changing its verdict.

```go
// Good — classify first, then mutate.
if allCardImportErrorsSkipped(parseErrs) {
    return SyncFromNotionOutput{ParseErrors: parseErrs}, nil
}
rows, parseErrs = dedupeParsedRows(rows, parseErrs)  // may append non-skip warnings

// Bad — dedupe runs first and adds "duplicate front" warnings (Kind: "DUPLICATE").
// allCardImportErrorsSkipped then returns false for a genuinely skip-only payload.
rows, parseErrs = dedupeParsedRows(rows, parseErrs)
if allCardImportErrorsSkipped(parseErrs) { ... }
```

Reference: `backend/internal/usecase/notion_sync.go` — `Sync` method, the skip-only short-circuit comment.

## Why this matters

`dedupeParsedRows` appends "duplicate front" validation entries (`Kind: "DUPLICATE"`) to `parseErrs` as a side effect of deduplication. If the skip-only classifier runs after `dedupeParsedRows`, a payload that contained only lone-front/lone-back lines (all `Kind: "FRONT_ONLY"` or `"BACK_ONLY"`) will also contain the duplicate warnings, making `allCardImportErrorsSkipped` return `false` and falling through to the hard-failure branch — which deletes existing cards rather than preserving them.

## Generalisation

Any classifier-then-mutate pipeline shares this constraint:

1. **Read the slice** via the classifier.
2. **Only then** pass the slice to functions that can append, filter, or reorder it.

When the order constraint is non-obvious (because the classify call and the mutate call are far apart, or the mutate call has an innocuous name like "dedupe"), document the ordering invariant with a comment at the classify call site so future readers understand why the lines cannot be swapped.

## Variant: input-integrity check before policy classification on a partial-map lookup

A subtler form of the same constraint applies when the classifier consumes a partial-map lookup — typically a repository's `FindByIDs(ctx, ids) (map[ID]*T, error)` that silently returns a map smaller than `len(ids)` when some IDs are unknown. A policy-level classifier that iterates `ids` and reads `out[id]` sees a zero / `false` for the missing slot, which then silently flows into the policy verdict.

Concrete failure mode in `backend/internal/usecase/admin_user.go` (`EditUser` self-edit branch). Before the fix:

```go
roles, _ := u.roles.FindByIDs(ctx, roleIDs)  // partial map for unknown IDs
keepsAdmin := false
for _, roleID := range roleIDs {
    if role, ok := roles[roleID]; ok && role.Name == domain.AdminRoleName {
        keepsAdmin = true
        break
    }
}
if !keepsAdmin {
    return AdminEditUserOutcome{CannotRevokeOwnAdmin: true}, nil  // wrong outcome for unknown roleId
}
```

When the caller sends an unknown roleId for a self-edit, `roles[id]` is absent, `keepsAdmin` stays `false`, and the caller sees `CannotRevokeOwnAdmin` — a misleading classification of "you cannot revoke your own admin role" when the real issue is "you sent an invalid role ID". The fix is to validate input-integrity before the policy loop:

```go
roles, _ := u.roles.FindByIDs(ctx, roleIDs)
if len(roles) != len(roleIDs) {
    return AdminEditUserOutcome{
        Validation: NewInputValidationInfo("roleIds", "role not found"),
    }, nil
}
// keepsAdmin loop runs only when every input ID resolved.
```

The generalisation: whenever a classifier or policy decision iterates an input slice and looks each element up in a partial-map return, the call site MUST first verify `len(input) == len(returnedMap)` (or filter to known IDs) before computing the policy outcome. Otherwise unknown inputs masquerade as the absence of a property and trigger the wrong classifier branch.

Two preconditions for the `len`-equality check to be meaningful:
- The input slice is deduplicated upstream (`normalizeAdminEditRoleIDs` filters duplicates and empty strings before the lookup). If the input can contain duplicates, the map will be shorter even when every ID resolves.
- The partial-map return convention is documented on the repository method. `FindByIDs` returns a map keyed on found IDs; unknown IDs are absent. Without this contract, the `len` check is a fragile heuristic.
