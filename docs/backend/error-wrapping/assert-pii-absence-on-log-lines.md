# Assert PII absence on log lines that carry `user_id`

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

Logs that intentionally include a stable identifier (`user_id`, `cardgroup_id`) and intentionally omit PII (`email`, `display_name`) need a CI-enforced floor on the omission, not just the inclusion. A future contributor adding `slog.String("email", u.Email)` for "easier triage" silently regresses the redaction policy unless the test fails. Pair every "field is present" assertion with a "field is absent" check on the same record:

```go
if rec0["user_id"] != wantUserID { t.Errorf(...) }
if _, hasEmail := rec0["email"]; hasEmail {
    t.Error("WARN log must not contain 'email' field (PII protection)")
}
```

Used in the `superuser_test.go` INFO and WARN cases — the policy in `auth/superuser.go` is "log `user_id` only", and the absence-tests are what hold that contract.
