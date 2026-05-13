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

## CLI output: extend the same rule to unstructured stdout/stderr

The slog-scoped rule above targets structured JSON fields in the application server. The same intent applies to unstructured output from CLI tools:

- Email addresses must not appear in `log.Printf` lines from CLI tools. Use `<redacted>` as the placeholder literal when an identifier must appear for diagnostic purposes.
- Summary lines that indicate skipped or failed items must print counts, not the actual email values:
  ```go
  // CORRECT
  log.Printf("skipped %d users (already seeded)", skippedCount)

  // WRONG — exposes PII in terminal output and log files
  log.Printf("skipped users: %v", emailList)
  ```
- `fmt.Printf` progress output follows the same rule: print row counts or UUID prefixes, never full email addresses.

This is a separate concern from the structured-field absence test above — both rules must be satisfied independently. A CLI that omits email from its slog JSON but prints it via `log.Printf` still violates the redaction policy.
