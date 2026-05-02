# Go library gotchas (backend)

> Applies to: `backend/internal/`, `backend/cmd/`. These are library-quirk rules — counter-intuitive behaviours of `uuid`, Echo v5, GORM, `golang-jwt/v5`, `slog`, and `crypto/subtle` that must be respected anywhere the library is touched.

## `uuid.NewV7` failure must propagate

`uuid.NewV7()` fails only when `crypto/rand` is unavailable, meaning the system is already unhealthy. A silent fallback to `uuid.NewV4()` is not safe — `NewV4` calls the same random source and will also fail. Helper functions that generate IDs must return `(string, error)` and let callers map the failure to `gqlerr.Internal`. The request-ID middleware is the one deliberate exception because it uses a timestamp string as fallback; that is specific to logging context, not business-logic IDs.

## `defer recover()` must re-panic `runtime.Error`

A blanket `recover()` in a `defer` collapses two unrelated failure modes into one user-facing parse error: a real bug like a nil-deref or index-out-of-range (`runtime.Error`) and a legitimate "we asked the parser to give up on this input" panic raised by hand. The runtime.Error case is a server bug and must surface in tests, CI, and crash reports — not get rewritten as `ValidationError`. The textdic parser uses this guard:

```go
defer func() {
    if r := recover(); r != nil {
        if rt, ok := r.(runtime.Error); ok {
            panic(rt)
        }
        err = fmt.Errorf("textdic: parser panic: %v\n%s", r, debug.Stack())
    }
}()
```

Apply the same shape to any new `recover` site that wraps third-party generated code (goyacc parsers, regex engines, text-processing libraries). The `debug.Stack()` capture is mandatory: by the time the recovered error is logged, the original goroutine stack is gone.

## Echo v5 handler signature uses a pointer receiver

Echo v5 handler and middleware signatures changed from v4. Every handler and middleware factory must use `*echo.Context` (pointer), not the v4 interface form:

```go
// v5 — correct
func handler(c *echo.Context) error { ... }

// v4 — will not compile or will behave wrong in v5
func handler(c echo.Context) error { ... }
```

Online samples, AI-generated code, and the official Echo v4 docs all use the interface form. Any paste from those sources requires this fix.

The type is `*echo.Context` — a **pointer to a concrete struct**, not an interface. v5 removed the `echo.Context` interface entirely, so there is no interface to embed or assert against.

## `echo.NewHTTPError` discards manually-set response headers

Echo's default `HTTPErrorHandler` serializes the error and writes a fresh response, discarding any headers set on the context before returning the error. Setting `c.Response().Header().Set("WWW-Authenticate", "...")` and then `return echo.NewHTTPError(401, "...")` will drop the header in the rendered response. The fix is to write the response directly and return `nil`:

```go
c.Response().Header().Set("WWW-Authenticate", `Bearer realm="api"`)
return c.String(http.StatusUnauthorized, "unauthorized")
```

Any future middleware that must send headers on an error response must use this pattern.

## JWT algorithm confusion: always whitelist valid algorithms

Without `jwt.WithValidMethods([]string{"ES256", "RS256"})`, an attacker can re-sign a token with `HS256` using the JWKS public key as the HMAC secret, or use `alg=none` to bypass signature verification entirely. `golang-jwt/v5` does not reject these by default if the keyfunc returns a key. Always pass `WithValidMethods` with the exact set of algorithms your JWKS endpoint issues.

## GORM `WHERE id IN ?` with an empty slice returns all rows

Passing an empty `[]string{}` to a GORM query like `db.Where("id IN ?", ids).Find(&rows)` does **not** emit `WHERE id IN ()`. GORM silently drops the clause and performs an unfiltered full-table scan, returning every row. Guard every batch-fetch path with an early return:

```go
if len(ids) == 0 {
    return nil, nil
}
```

This matters most in DataLoader batch functions, where an empty key slice is a normal edge case.

## GORM v1 string-typed primary key with DB-generated UUID requires `default:` tag

GORM's `BeforeCreate` hook auto-generates a UUID when a primary key field is a `string` type and is zero-valued — but only when the field carries `gorm:"default:..."` in its tag. Without the tag, GORM leaves the field empty and the INSERT fails.

Peer models (`gormUser`, `gormCard`) avoid this by supplying the UUID in the application layer before calling `Create`. `gormPingRecord` is different: `Create` inserts a row without any caller-supplied ID, so the DB must generate it via `gen_random_uuid()`. The tag `gorm:"default:gen_random_uuid()"` is therefore load-bearing even though `AutoMigrate` is not used and the column default is already defined in the migration SQL.

## GORM `LIKE` / `ILIKE` requires escaping `%`, `_`, `\` in user input

A search box that runs `WHERE name ILIKE ? || '%'` with a user-supplied string becomes a pattern-injection surface: a user typing `100%` matches every row containing the literal string `100`, not just rows starting with `100%`. The three Postgres `LIKE` metacharacters are `%`, `_`, and `\` (the default escape). User-supplied search text must be escaped before being wrapped with `%...%`:

```go
func escapeLike(s string) string {
    s = strings.ReplaceAll(s, `\`, `\\`)
    s = strings.ReplaceAll(s, `%`, `\%`)
    s = strings.ReplaceAll(s, `_`, `\_`)
    return s
}
// ...
db.Where("name ILIKE ?", "%"+escapeLike(query)+"%")
```

Order matters: escape `\` first, then `%` and `_`, otherwise the second pass re-escapes the backslash from the first pass. The same rule applies to `name ILIKE ? || '%'` (prefix match) and to any other `LIKE` predicate fed by user input.

## GORM rejects unconditional `Delete` — use `Where("1 = 1")` to opt out

GORM v2+ refuses a `Delete` call that has no `WHERE` clause as a safety net against accidental full-table deletes. It returns an `ErrMissingWhereClause` error.

The deliberate opt-out for legitimate full-table deletes is:

```go
result := db.Where("1 = 1").Delete(&gormPingRecord{})
```

This makes the intent explicit and satisfies GORM's guard without suppressing the error check.

## `subtle.ConstantTimeCompare` leaks token length — pair with a rate limiter

`crypto/subtle.ConstantTimeCompare` returns early (0) when the two byte slices differ in length, leaking length via timing. For equal-length inputs the comparison runs in constant time. The practical impact for bearer-token checking is small when the token length is public knowledge (e.g. a fixed 64-hex-char token), but the leak becomes meaningful for variable-length or secret-length tokens without an external mitigation.

The `/internal/ping` handler pairs `ConstantTimeCompare` with a per-IP rate limiter (1 req/s, burst 5). The rate limiter makes the length-oracle non-exploitable in practice: an attacker cannot iterate quickly enough to extract useful information before being throttled. Any future endpoint that adopts bearer-token comparison **without** a rate limiter must also add one — or switch to a constant-time scheme that does not branch on length.

## slog context enrichment must precede the log call that announces the enrichment

When a middleware sets a value in the context and then logs a message about
that action, the `slog.*Context` call must come **after**
`c.SetRequest(c.Request().WithContext(ctx))`. Logging before the context is
stored means the very line that announces the event carries no `request_id` (or
other context attribute) itself. The pattern in
`backend/internal/middleware/request_id.go` — enriching the context first, then
calling `slog.WarnContext(ctx, ...)` — is the correct template for any
context-enriched slog handler.

## Constructor panics are the right tool for "non-empty config requires non-nil deps"

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

## Go `map` is a reference type — copy in the constructor when accepting one

A constructor that stashes a caller-supplied `map` directly (`p.emails = emails`) leaves the invariant under the caller's control: any later `delete(emails, k)` or `emails[k] = struct{}{}` mutates the constructed object's internal state without going through any of its methods. This is silent and almost impossible to track down because the receiver has no API surface that names the violation. `slice` has the same property; the fix shape is the same.

```go
emailsCopy := make(map[string]struct{}, len(emails))
for k := range emails { emailsCopy[k] = struct{}{} }
return &SuperUserPromoter{ emails: emailsCopy, /* ... */ }
```

The copy is `O(n)` once at construction and the receiver's invariants are now tamper-proof. Apply to any constructor whose stored field is a reference type (`map`, `slice`, `chan`) and whose correctness depends on the contents being stable.

## `json:",omitempty"` controls marshal output, never the decode path

`omitempty` is a *marshal-side* directive: it tells `encoding/json.Marshal` to skip the field when its value is the zero value. It does **not** affect `Unmarshal`. A claim like `EmailVerified bool \`json:"email_verified,omitempty"\`` decodes a missing claim as the Go zero value (`false`) — the same as `bool` would do without the tag. This is desirable for security gates that should default-deny on a missing claim, but only when the design explicitly relies on that behaviour:

```go
type supabaseClaims struct {
    Email         string `json:"email,omitempty"`
    EmailVerified bool   `json:"email_verified,omitempty"`
    // missing claim => EmailVerified == false (zero value), not an error.
}
```

If the design requires distinguishing "claim absent" from "claim present and false", use `*bool` instead and check for nil. Either choice is fine; what is **not** fine is assuming `omitempty` does anything for the receiving direction. Asserted in `backend/internal/auth/superuser_test.go` (`TestSupabaseClaims_EmailVerified`).

## Echo middleware factory: build the no-op decision once, not per-request

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

Read the source-of-truth field directly (`len(p.emails) == 0`) rather than caching the decision in a derived `bool` on the struct — see "Derived flags drift" below.

## Derived flags drift; read the source of truth instead

A `passthrough bool` field on a struct that is "kept in sync with `len(emails) == 0`" introduces two states that the type system does not enforce to agree. Any future constructor variant, copy, or mutation path that sets one and forgets the other produces a struct whose hot-path branch disagrees with its data. The fix is to delete the cache and read the source of truth at the decision point:

```go
// AVOID: derived flag duplicates state already in p.emails.
type SuperUserPromoter struct { emails map[string]struct{}; passthrough bool /* derived */ }

// PREFER: compute on read; impossible to drift.
if len(p.emails) == 0 { /* pass-through branch */ }
```

The rule generalises to any field that is fully determined by another field on the same struct: prefer recomputation unless profiling shows the read is hot enough to matter. For an Echo middleware factory `Middleware()` that runs once per process (not per request), the cost is rounding-error.

## `slog.NewJSONHandler` renders attrs as JSON keys, not `key=value` pairs

The production logger in `backend/cmd/server/main.go` is `slog.NewJSONHandler(os.Stderr, ...)`. A call site that writes `logger.Info("super-user bootstrap enabled", "email_count", n)` therefore lands in the log stream as `{"msg":"super-user bootstrap enabled","email_count":3}`, **not** the `slog.NewTextHandler` shape `msg="super-user bootstrap enabled" email_count=3` that test stubs and quick-reproduce snippets often print. Operator-facing docs that quote a log line for grep instructions must quote the JSON shape — instructing an operator to grep for `email_count=N` against a JSON-handler stream produces zero matches. The failure mode that exposed this rule: a runbook said "look for `email_count=3`" while production emitted `"email_count":3`, and the operator gave up after `grep` returned nothing. The rule applies symmetrically to any future operator runbook that embeds a log-line excerpt — match the rendering of whichever handler the relevant `main()` constructs.

## `slog.Handler.WithGroup` nests subsequent attrs inside the group object

Calling `handler.WithGroup("g")` on a `slog.JSONHandler` (or any handler that
wraps one, such as `logging.ContextHandler`) causes **all** attrs added
afterward — including those injected by `Handle` via `r.AddAttrs` — to appear
under the `"g"` JSON key, not at the top level. In `logging.ContextHandler`,
`request_id` is added via `r.AddAttrs` inside `Handle`, so after
`WithGroup("grp")` the log line becomes `{"grp":{"request_id":"...","k":"v"}}`.
Log queries and tests that expect top-level `request_id` will miss it.
`backend/internal/logging/handler_test.go` (`TestContextHandler_WithAttrsAndWithGroupPreserveRequestID`)
documents and asserts this shape.
