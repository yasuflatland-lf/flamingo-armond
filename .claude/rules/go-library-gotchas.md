# Go library gotchas (backend)

> Applies to: `backend/internal/`, `backend/cmd/`. These are library-quirk rules — counter-intuitive behaviours of `uuid`, Echo v5, GORM, `golang-jwt/v5`, `slog`, and `crypto/subtle` that must be respected anywhere the library is touched.

## `uuid.NewV7` failure must propagate

`uuid.NewV7()` fails only when `crypto/rand` is unavailable, meaning the system is already unhealthy. A silent fallback to `uuid.NewV4()` is not safe — `NewV4` calls the same random source and will also fail. Helper functions that generate IDs must return `(string, error)` and let callers map the failure to `gqlerr.Internal`. The request-ID middleware is the one deliberate exception because it uses a timestamp string as fallback; that is specific to logging context, not business-logic IDs.

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

## Detailed cases (on-demand)

- [`defer recover()` must re-panic `runtime.Error`](../../docs/backend/library-gotchas/defer-recover-must-rethrow-runtime-error.md)
- [`echo.NewHTTPError` discards manually-set response headers](../../docs/backend/library-gotchas/echo-newhttperror-discards-headers.md)
- [GORM v1 string-typed primary key with DB-generated UUID requires `default:` tag](../../docs/backend/library-gotchas/gorm-string-pk-default-tag.md)
- [GORM `LIKE` / `ILIKE` requires escaping `%`, `_`, `\` in user input](../../docs/backend/library-gotchas/gorm-like-ilike-escape.md)
- [GORM exact-match `FindBy*` helpers: callers own trimming, repos own nothing](../../docs/backend/library-gotchas/gorm-exact-match-findby-trimming.md)
- [Nullable filter fields: normalize `nil` / empty / whitespace at the usecase boundary](../../docs/backend/library-gotchas/nullable-filter-normalize-at-usecase.md)
- [Repository lookup methods scoped by tenant ID require a cross-tenant negative test](../../docs/backend/library-gotchas/repository-cross-tenant-negative-test.md)
- [GORM rejects unconditional `Delete` — use `Where("1 = 1")` to opt out](../../docs/backend/library-gotchas/gorm-rejects-unconditional-delete.md)
- [Postgres advisory lock for race-safe ensure-by-name when no UNIQUE constraint exists](../../docs/backend/library-gotchas/postgres-advisory-lock-for-ensure-by-name.md)
- [`subtle.ConstantTimeCompare` leaks token length — pair with a rate limiter](../../docs/backend/library-gotchas/subtle-constanttimecompare-length-leak.md)
- [slog context enrichment must precede the log call that announces the enrichment](../../docs/backend/library-gotchas/slog-context-enrichment-precedes-log.md)
- [Constructor panics are the right tool for "non-empty config requires non-nil deps"](../../docs/backend/library-gotchas/constructor-panics-for-non-empty-config.md)
- [Go `map` is a reference type — copy in the constructor when accepting one](../../docs/backend/library-gotchas/go-map-reference-copy-in-constructor.md)
- [`json:",omitempty"` controls marshal output, never the decode path](../../docs/backend/library-gotchas/json-omitempty-marshal-only.md)
- [Extending a JSON-marshaled struct: tag all fields, not just the new ones](../../docs/backend/library-gotchas/json-tag-asymmetry-on-struct-extension.md)
- [Echo middleware factory: build the no-op decision once, not per-request](../../docs/backend/library-gotchas/echo-middleware-factory-once-at-construction.md)
- [Derived flags drift; read the source of truth instead](../../docs/backend/library-gotchas/derived-flags-drift.md)
- [`slog.NewJSONHandler` renders attrs as JSON keys, not `key=value` pairs](../../docs/backend/library-gotchas/slog-jsonhandler-shape.md)
- [`slog.Handler.WithGroup` nests subsequent attrs inside the group object](../../docs/backend/library-gotchas/slog-withgroup-nests-attrs.md)
- [Embed a `panic` base struct to eliminate interface-stub boilerplate](../../docs/backend/library-gotchas/panic-base-struct-for-interface-stubs.md)
- [Extract startup helpers to make branch coverage testable without a live server](../../docs/backend/library-gotchas/extract-startup-helpers-for-branch-coverage.md)
- [Inline copy of production logic in tests is an anti-pattern](../../docs/backend/library-gotchas/inline-copy-of-production-logic-in-tests.md)
- [Method dispatch on a nil pointer panics — `u == nil` guards in methods are unreachable](../../docs/backend/library-gotchas/method-dispatch-nil-receiver-unreachable.md)
- [Test stubs `t.Fatalf` on exhausted fixture access, never panic](../../docs/backend/library-gotchas/test-stub-fatal-on-exhausted-fixture.md)
- [Optional feature: pass `nil` handler and let the router skip route registration](../../docs/backend/library-gotchas/optional-feature-nil-handler-skip-route.md)
- [goyacc lexer: recover via NEWLINE to enable `error NEWLINE` grammar rules](../../docs/backend/library-gotchas/goyacc-lexer-recovery-via-newline.md)
- [gqlgen `transport.POST` response headers must be set at construction time](../../docs/backend/library-gotchas/gqlgen-transport-post-response-headers.md)
- [Panic value format: `%T %v` vs `%T`-only — PII trade-off](../../docs/backend/library-gotchas/panic-value-format-pii-tradeoff.md)
- [XOR-invariant outcome structs for mutually-exclusive results](../../docs/backend/library-gotchas/xor-invariant-outcome-struct.md)
