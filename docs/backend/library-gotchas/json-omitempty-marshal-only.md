# `json:",omitempty"` controls marshal output, never the decode path

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

`omitempty` is a *marshal-side* directive: it tells `encoding/json.Marshal` to skip the field when its value is the zero value. It does **not** affect `Unmarshal`. A claim like `EmailVerified bool \`json:"email_verified,omitempty"\`` decodes a missing claim as the Go zero value (`false`) — the same as `bool` would do without the tag. This is desirable for security gates that should default-deny on a missing claim, but only when the design explicitly relies on that behaviour:

```go
type supabaseClaims struct {
    Email         string `json:"email,omitempty"`
    EmailVerified bool   `json:"email_verified,omitempty"`
    // missing claim => EmailVerified == false (zero value), not an error.
}
```

If the design requires distinguishing "claim absent" from "claim present and false", use `*bool` instead and check for nil. Either choice is fine; what is **not** fine is assuming `omitempty` does anything for the receiving direction. Asserted in `backend/internal/auth/superuser_test.go` (`TestSupabaseClaims_EmailVerified`).
