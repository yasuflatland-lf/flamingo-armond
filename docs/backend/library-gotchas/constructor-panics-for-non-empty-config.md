# Constructor panics are the right tool for "non-empty config requires non-nil deps"

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a constructor accepts a feature-flag-shaped configuration plus the dependencies that are required *only* when the flag is non-empty, returning an error is awkward (every caller has to plumb an extra error through `run()`) and a silent half-configured struct is a per-request nil-deref hazard. Panic at construction is the right level of force: the misconfiguration is an operator-visible programming error, not a runtime input, and `run()` has not yet started the HTTP server when it fires — Echo's `Recover` middleware is not in the path, so the panic crashes the process at boot.

```go
func NewSuperUserPromoter(emails map[string]struct{}, adminRoleID string,
    checker adminChecker, assigner roleAssigner) *SuperUserPromoter {
    if len(emails) > 0 {
        if checker == nil      { panic("auth: ... checker must not be nil when emails is non-empty") }
        if assigner == nil     { panic("auth: ... assigner must not be nil when emails is non-empty") }
        if adminRoleID == ""   { panic("auth: ... adminRoleID must not be empty when emails is non-empty") }
    }
    // empty-emails path: nil deps are intentional; Middleware() returns a pass-through.
}
```

The pattern only applies to config-shaped constructors where one branch (here, the OFF branch) legitimately accepts zero values. Constructors whose contract is "always need these deps" should use a regular nil-check + return-error.
