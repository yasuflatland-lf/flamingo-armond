# Echo middleware factory: build the no-op decision once, not per-request

> Part of the [Go library gotchas](../../../.claude/rules/go-library-gotchas.md) rules.

When a middleware has a feature-flag branch ("do something when configured, otherwise pass through"), evaluate the flag once at construction time and return a different function from the factory. A `len(p.emails) == 0` check inside the per-request closure runs on every request even when the feature is off; lifting it out of the closure makes the OFF path a literal `func(next) { return next }` and the inliner can optimise the call entirely:

```go
func (p *SuperUserPromoter) Middleware() echo.MiddlewareFunc {
    if len(p.emails) == 0 {
        return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
    }
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c *echo.Context) error { /* full hot path */ }
    }
}
```

Read the source-of-truth field directly (`len(p.emails) == 0`) rather than caching the decision in a derived `bool` on the struct — see [Derived flags drift; read the source of truth instead](derived-flags-drift.md).
