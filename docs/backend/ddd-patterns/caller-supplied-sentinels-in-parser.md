# Caller-supplied sentinels in a field-agnostic parser

> Part of the [DDD patterns](./../../../.claude/rules/ddd-patterns.md) rules.

## Why

`Card` has two text fields — `Front` and `Back` — with identical validation rules
but different domain sentinels (`ErrCardFrontRequired` vs `ErrCardBackRequired`).
Before this PR, `usecase/card.go` contained a `validateCardText(field, value string)`
function that dispatched on a `field == "front"` string comparison. The problems:

- The discrimination is fragile: a misspelling compiles without complaint.
- Each field's validation is not symmetric — a new field requires a new branch.
- The VO (`CardText`) knew nothing about which sentinel to return, so the field
  identity lived outside the VO and had to be kept in sync manually.

## What

`ParseCardText` accepts the two sentinels as parameters:

```go
// domain/card_text.go

func ParseCardText(s string, requiredErr, tooLongErr error) (CardText, error) {
    if requiredErr == nil || tooLongErr == nil {
        panic("domain: ParseCardText requires non-nil requiredErr and tooLongErr sentinels")
    }
    trimmed := strings.TrimSpace(s)
    n := uniseg.GraphemeClusterCount(trimmed)
    if n < 1 {
        return "", requiredErr
    }
    if n > CardTextMax {
        return "", tooLongErr
    }
    return CardText(trimmed), nil
}
```

The VO stays field-agnostic. The aggregate constructor (`NewCard`) picks the
right sentinel for each field:

```go
// domain/card.go

func NewCard(cardgroupID, front, back string, position int) (*Card, error) {
    // ...
    if _, err := ParseCardText(front, ErrCardFrontRequired, ErrCardFrontTooLong); err != nil {
        return nil, err
    }
    if _, err := ParseCardText(back, ErrCardBackRequired, ErrCardBackTooLong); err != nil {
        return nil, err
    }
    // ... generate ID, stamp timestamps, return &Card{...}, nil
}
```

The sentinels themselves (`ErrCardFrontRequired`, etc.) are declared at the `domain`
level and are available to the usecase layer's `translate*Err` helpers.

## Panic on nil sentinels

`ParseCardText` panics when either sentinel is `nil`. This is a programmer-error
guard: nil sentinels cause the function to return `nil` errors on validation
failure, which the caller interprets as "valid" — a silent correctness bug. A panic
surfaces the mistake immediately in tests. This mirrors the `ucerr.NewValidationError`
convention documented in
[`docs/backend/error-wrapping/input-validation-info-empty-field-panic.md`](../error-wrapping/input-validation-info-empty-field-panic.md).
