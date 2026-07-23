# Patch DTOs keep primitive types, not the VO

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

A patch DTO (e.g. `repository.UserUpdate`, `repository.CardUpdate`) is a thin
data carrier between the usecase and the repository's `Updates(map)` call.
Each field expresses the patch contract directly:

| DTO field shape | Meaning |
|---|---|
| `nil` pointer | "leave the stored value unchanged" |
| pointer to `""` | "clear the column" |
| pointer to `"x"` | "set the column to `\"x\"`" |

The primitive `*string` expresses this contract literally — `nil` is the
absence of a patch, anything non-`nil` is a write. Retyping the field to a
struct VO (e.g. `Bio` instead of `*string`) does not add safety here:

- The patch contract is **about presence/absence**, not about the validated
  shape of the value. The struct VO's invariants (length cap, etc.) were
  already enforced by the upstream `Parse*` call in the usecase.
- The repository's `Updates(map)` site takes `any` values and writes them
  verbatim. A struct VO would have to be translated back to `*string` at
  this site (`if bio.IsSet() { updates["bio"] = *bio.Ptr() }`), adding two
  lines per field for no semantic gain.
- The DTO retains its role as a *data carrier* — it does not validate, it
  does not enforce, it just describes the patch.

The boundary between "value object" and "patch DTO" is the usecase function:
the VO lives on the way in (validated user input), the DTO lives on the way
out (write request to the repository).

## What — `UserUpdate.Bio`

`repository.UserUpdate.Bio` is `*string`, even though the domain field
`User.Bio` is the struct VO `Bio`. The usecase translates the validated `Bio`
back to `*string` at the patch construction site. That construction site is the
shared `buildUserProfilePatch` helper in `backend/internal/usecase/user.go`,
used by both `userUsecase.UpdateUser` (self-service) and
`adminUserUsecase.EditUser` (admin):

```go
// backend/internal/usecase/user.go — buildUserProfilePatch
func buildUserProfilePatch(displayName *string, bio *string) (repository.UserUpdate, *InputValidationInfo, error) {
    patch := repository.UserUpdate{}
    if displayName != nil {
        // ... liftValidationErr / translateDisplayNameErr branch
        patch.DisplayName = &name
    }
    if bio != nil {
        b, err := domain.ParseBio(bio)
        if err != nil {
            // ... liftValidationErr / translateBioErr branch
        }
        patch.Bio = b.Ptr()   // <- VO → primitive at the DTO boundary
    }
    return patch, nil, nil
}
```

The `if bio != nil` guard is the patch contract's "field absent" branch:
the input field is a `*string` because the input itself is a patch (the GraphQL
mutation `UpdateProfileInput.Bio` is `*string`). Keeping the same shape on the
DTO side keeps the translation trivial.

If `UserUpdate.Bio` were retyped to `Bio`, the repository site would have to
demote it back:

```go
// HYPOTHETICAL — what the retyping would force at the repository.
if patch.Bio.IsSet() {
    updates["bio"] = *patch.Bio.Ptr()
}
```

…with no validation, no extra safety, just a per-field demote. The primitive
shape keeps the repository unaware of the VO.

## What — `CardUpdate.Front` / `CardUpdate.Back`

The same shape applies to `repository.CardUpdate`: `Front` and `Back` are
`*string`, even though `Card.Front` / `Card.Back` are `CardText`. The usecase
validates via `ParseCardText` and translates to `*string` at the DTO
construction site:

```go
// backend/internal/usecase/card.go — CardUsecase.Update
if in.Front != nil {
    front, err := domain.ParseCardText(*in.Front, domain.ErrCardFrontRequired, domain.ErrCardFrontTooLong)
    if err != nil { /* ... validation branch */ }
    s := front.String()      // <- VO → primitive at the DTO boundary
    patch.Front = &s
}
```

The `.String()` method is what makes the cast explicit at the call site. It is
the only place the `CardText.String()` accessor is wired today (two callers
in `card.go`), but it is load-bearing — without it the patch construction
would have to use `(*string)(&front)` casts or convert via intermediate
variables, both of which read worse than the named accessor.

## When the pattern does NOT apply

- **Constructor DTOs** (`CreateUserInput`) — these are inputs, not patches.
  The "no change" semantics do not exist; every field has a definite value.
  Retyping to the VO (`DisplayName string` → `DisplayName DisplayName`) is
  legitimate and arguably preferable, though this codebase keeps the
  primitives at the input boundary and validates inside the usecase.
- **Single-field patches** that already use a typed enum or boolean —
  `RoleUpdate.Name string` becomes `RoleName RoleName` because the field's
  only valid values are the validated newtype; the primitive shape adds no
  information.

The rule is: keep the DTO shape aligned with the contract the DTO encodes.
For three-way patch DTOs, `*<primitive>` matches the contract. For
two-way constructor DTOs, the VO matches the contract.

## String-enum patch fields require a wired validation seam

The primitive patch convention can apply to a string-enum newtype, but only
when a production usecase validates the value before constructing the patch.
The repository's `Updates(map)` call and a database `CHECK` constraint do not
replace that usecase boundary.

`MasterCardgroupUpdate.Status` was previously documented as an example, but it
had no production writer or validation seam. Keeping the exported field allowed
callers to bypass the aggregate's publication state machine, so the field was
removed. Status and version transitions now remain exclusively behind
`MasterCardgroupRepository.Publish` and `MasterCardgroupRepository.Unpublish`,
which delegate the transition rules to `MasterCardgroup.Publish` and
`MasterCardgroup.Unpublish`.

For a future string-enum patch field, first establish that patching is part of
the aggregate's contract. Validate the proposed value at the usecase boundary,
then pass the primitive representation through the patch DTO. If the value has
dedicated lifecycle operations or aggregate transition rules, expose those
operations instead of a general patch field.

## Reference

- `backend/internal/repository/user.go` — `UserUpdate` (`*string` for `DisplayName`/`Bio`/`AvatarURL`).
- `backend/internal/repository/card.go` — `CardUpdate` (`*string` for `Front`/`Back`).
- `backend/internal/usecase/user.go` — `buildUserProfilePatch` translates `Bio.Ptr()` to `patch.Bio *string` for both `UpdateUser` and `adminUserUsecase.EditUser`.
- `backend/internal/usecase/card.go` — `CardUsecase.Update` translates `CardText.String()` to `patch.Front/Back *string`.
- [`docs/backend/ddd-patterns/trinary-value-object.md`](trinary-value-object.md) — the `Bio` VO's own contract.
- [`docs/backend/ddd-patterns/context-neutral-vo-docstrings.md`](context-neutral-vo-docstrings.md) — the patch-vs-read context split for shared VOs.
