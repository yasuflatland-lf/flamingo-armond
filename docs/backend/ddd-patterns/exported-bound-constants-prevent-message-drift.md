# Exported bound constants prevent message drift

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

If the validation cap is an unexported constant inside the domain package and the
usecase error message hardcodes the literal (`"50 characters"`), the cap and the
message can drift independently:

- Changing `roleNameMax` from 50 to 100 inside `domain` still compiles.
- The usecase message still says "50 characters" until someone remembers to
  update it — and they usually do not.

The mismatch is invisible in tests unless the test pins the exact message string.

## What

Export the constant even though only the domain layer enforces the bound:

```go
// domain/role_name.go
const RoleNameMax = 50

// domain/display_name.go
const DisplayNameMax = 50

// domain/bio.go
const BioMax = 500
```

The usecase `translate*Err` helper references the constant when formatting the
message:

```go
// usecase/admin_role.go

func translateRoleNameErr(err error) error {
    // ...
    case errors.Is(err, domain.ErrRoleNameTooLong):
        return ucerr.NewValidationError("name",
            fmt.Sprintf("name must be at most %d characters", domain.RoleNameMax))
    // ...
}
```

```go
// usecase/admin_user.go

func translateDisplayNameErr(err error) error {
    // ...
    case errors.Is(err, domain.ErrDisplayNameTooLong):
        return ucerr.NewValidationError("displayName",
            fmt.Sprintf("displayName must be at most %d characters", domain.DisplayNameMax))
    // ...
}

func translateBioErr(err error) error {
    // ...
    if errors.Is(err, domain.ErrBioTooLong) {
        return ucerr.NewValidationError("bio",
            fmt.Sprintf("bio must be at most %d characters", domain.BioMax))
    }
    // ...
}
```

The cap lives in one place (`domain/*.go`); the message is derived from it at the
`translate*Err` call site. Updating the cap automatically propagates to the
user-facing message.

## Rule of thumb

Any constant that appears in a domain sentinel's message text (e.g. `eris.Errorf("role:
name exceeds %d characters", RoleNameMax)`) must be exported. If the constant is
used only in unexported sentinel formatting inside the domain package, it could stay
unexported — but the sentinel's presence on the wire (via `translate*Err`) makes the
cap effectively public, and exporting the symbol makes that fact explicit.
