# GORM rejects unconditional `Delete` — use `Where("1 = 1")` to opt out

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

GORM v2+ refuses a `Delete` call that has no `WHERE` clause as a safety net against accidental full-table deletes. It returns an `ErrMissingWhereClause` error.

The deliberate opt-out for legitimate full-table deletes is:

```go
result := db.Where("1 = 1").Delete(&gormPingRecord{})
```

This makes the intent explicit and satisfies GORM's guard without suppressing the error check.
