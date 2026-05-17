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
- [Tx and non-Tx repository methods share a private helper to avoid drift](../../docs/backend/library-gotchas/repo-tx-and-nontx-share-private-helper.md)
- [Consumer-defined narrow repository interface per usecase](../../docs/backend/library-gotchas/consumer-defined-narrow-repo-interface.md)
- [Inject `*rand.Rand` into pure functions to keep tests deterministic](../../docs/backend/library-gotchas/inject-rand-rand-for-deterministic-test.md)
- [Postgres advisory lock for race-safe ensure-by-name when no UNIQUE constraint exists](../../docs/backend/library-gotchas/postgres-advisory-lock-for-ensure-by-name.md)
- [`subtle.ConstantTimeCompare` leaks token length — pair with a rate limiter](../../docs/backend/library-gotchas/subtle-constanttimecompare-length-leak.md)
- [slog context enrichment must precede the log call that announces the enrichment](../../docs/backend/library-gotchas/slog-context-enrichment-precedes-log.md)
- [Constructor panics are the right tool for "non-empty config requires non-nil deps"](../../docs/backend/library-gotchas/constructor-panics-for-non-empty-config.md)
- [Constructor panics must cover argument-relationship invariants, not just nil checks](../../docs/backend/library-gotchas/constructor-relationship-invariant-panic.md)
- [Variadic optional dep injection is an antipattern in constructors](../../docs/backend/library-gotchas/variadic-optional-dep-injection-antipattern.md)
- [Docblock correction cascades — audit rule prose + bullet list on same edit](../../docs/backend/library-gotchas/constructor-docblock-cascade.md)
- [Go `map` is a reference type — copy in the constructor when accepting one](../../docs/backend/library-gotchas/go-map-reference-copy-in-constructor.md)
- [`json:",omitempty"` controls marshal output, never the decode path](../../docs/backend/library-gotchas/json-omitempty-marshal-only.md)
- [Extending a JSON-marshaled struct: tag all fields, not just the new ones](../../docs/backend/library-gotchas/json-tag-asymmetry-on-struct-extension.md)
- [Echo middleware factory: build the no-op decision once, not per-request](../../docs/backend/library-gotchas/echo-middleware-factory-once-at-construction.md)
- [Derived flags drift; read the source of truth instead](../../docs/backend/library-gotchas/derived-flags-drift.md)
- [`slog.NewJSONHandler` renders attrs as JSON keys, not `key=value` pairs](../../docs/backend/library-gotchas/slog-jsonhandler-shape.md)
- [`slog.Handler.WithGroup` nests subsequent attrs inside the group object](../../docs/backend/library-gotchas/slog-withgroup-nests-attrs.md)
- [Embed a `panic` base struct to eliminate interface-stub boilerplate](../../docs/backend/library-gotchas/panic-base-struct-for-interface-stubs.md)
- [Testable startup helpers — anti-pattern of inline test copies and the extract-helper fix](../../docs/backend/library-gotchas/testable-startup-helpers.md)
- [Method dispatch on a nil pointer panics — `u == nil` guards in methods are unreachable](../../docs/backend/library-gotchas/method-dispatch-nil-receiver-unreachable.md)
- [Test stubs `t.Fatalf` on exhausted fixture access, never panic](../../docs/backend/library-gotchas/test-stub-fatal-on-exhausted-fixture.md)
- [Optional feature: pass `nil` handler and let the router skip route registration](../../docs/backend/library-gotchas/optional-feature-nil-handler-skip-route.md)
- [goyacc lexer: recover via NEWLINE to enable `error NEWLINE` grammar rules](../../docs/backend/library-gotchas/goyacc-lexer-recovery-via-newline.md)
- [gqlgen `transport.POST` response headers must be set at construction time](../../docs/backend/library-gotchas/gqlgen-transport-post-response-headers.md)
- [gqlgen wraps deleted-field resolvers in a `// !!! WARNING !!!` block — they are not auto-removed](../../docs/backend/library-gotchas/gqlgen-warning-block-on-deleted-resolver.md)
- [Panic value format: `%T %v` vs `%T`-only — PII trade-off](../../docs/backend/library-gotchas/panic-value-format-pii-tradeoff.md)
- [XOR-invariant outcome structs for mutually-exclusive results](../../docs/backend/library-gotchas/xor-invariant-outcome-struct.md)
- [Fire-and-forget goroutine: detach context from request lifecycle](../../docs/backend/library-gotchas/fire-and-forget-goroutine-detached-context.md)
- [Channel-based "never called" assertion via `select` + `time.After`](../../docs/backend/library-gotchas/channel-based-never-called-assertion.md)
- [Transactional read-modify-write must use the `tx` handle, not `r.db`](../../docs/backend/library-gotchas/gorm-tx-read-modify-write.md)
- [GORM `Updates(map) + RowsAffected + FindByID` for race-free single-field updates (replaces `Take + Save`)](../../docs/backend/library-gotchas/gorm-update-via-updates-rowsaffected-findbyid.md)
- [Int-typed domain enums need `IsValid()` on DB reconstitution](../../docs/backend/library-gotchas/gorm-enum-cast-isvalid.md)
- [GraphQL resolver: synthesized domain values must be deterministic](../../docs/backend/library-gotchas/graphql-resolver-stable-synthesis.md)
- [Raw SQL CLI: `sql.Open("pgx", dbURL)` + pgx stdlib, not GORM; validate DSN before Open; auth.users needs superuser DSN](../../docs/backend/library-gotchas/raw-sql-cli-pgx-stdlib.md)
- [`*sql.Rows`: explicit `rows.Close()` after loop in addition to `defer rows.Close()`](../../docs/backend/library-gotchas/sql-rows-explicit-close.md)
- [Named return `(retErr error)` for deferred `tx.Rollback` — local `err` can be shadowed](../../docs/backend/library-gotchas/named-return-deferred-rollback.md)
- [Sensitive file output: use `0o600`, not `0o644`, for files containing PII](../../docs/backend/library-gotchas/sensitive-file-permissions-0o600.md)
- [`t.Parallel()` + `slog.SetDefault()` mutation is a data race — capture `prev` and restore in `t.Cleanup`](../../docs/backend/library-gotchas/tparallel-slog-setdefault-race.md)
- [Unexported types must not have exported fields](../../docs/backend/library-gotchas/unexported-type-exported-fields.md)
- [Consolidate env-var reads into a typed config struct + factory (`serverConfig` pattern)](../../docs/backend/library-gotchas/server-config-typed-env-struct.md)
- [`t.Setenv` vs `os.Setenv` in tests — `t.Setenv` auto-restores; `os.Setenv` leaks](../../docs/backend/library-gotchas/tsetenv-vs-os-setenv.md)
- [Tx and non-Tx repository methods must return the same shape (empty container, never `nil`) on empty input](../../docs/backend/library-gotchas/tx-nontx-empty-input-return-symmetry.md)
- [Log output assertions: always read the buffer or the assertion is dead code](../../docs/backend/library-gotchas/slog-buffer-assertions-must-be-checked.md)
- [`errors.AsType[T error]` — Go 1.26 generic narrowing helper that scopes the captured value to the `if` block](../../docs/backend/library-gotchas/errors-astype-generic-helper.md)
- [Extracting a sibling aggregate when a field is "about X" rather than "part of X"](../../docs/backend/library-gotchas/sibling-aggregate-extraction.md)
- [Ownership-checked UPSERT: collapse `EXISTS` ownership predicate into the INSERT statement](../../docs/backend/library-gotchas/ownership-checked-upsert-where-exists.md)
- [DataLoader two-step chain: hydrate aggregate A, then key aggregate B from its field](../../docs/backend/library-gotchas/dataloader-two-step-chain.md)
- [DataLoader missing-key semantics: `nil data` vs `ErrNotFound` is per-aggregate](../../docs/backend/library-gotchas/dataloader-missing-key-semantics.md)
- [`ON DELETE` FK action requires an integration test against a real database](../../docs/backend/library-gotchas/fk-action-integration-test.md)
- [Migration down/up roundtrip test: prove the reverse path preserves data](../../docs/backend/library-gotchas/migration-down-up-roundtrip-test.md)
- [RLS `INSERT-own` assertion on a PK-keyed table needs a fixture row that does not exist yet](../../docs/backend/library-gotchas/rls-insert-own-fresh-fixture-row.md)
- [LSP stale-cache diagnostics during refactor — verify with `go build` before treating as real](../../docs/backend/library-gotchas/lsp-stale-cache-during-refactor.md)
- [CI bash `rc=$?` is dead code under `set -e` — use `cmd || rc=$?`](../../docs/backend/library-gotchas/ci-bash-rc-capture-under-set-e.md)
- [Walker / parser positive-discovery guards — silent zero disables enforcement](../../docs/backend/library-gotchas/walker-parser-positive-discovery-guards.md)
- [Resolver-injected usecase: interface field unlocks unit tests for outcome-union guards](../../docs/backend/library-gotchas/resolver-usecase-interface-vs-concrete-testability.md)
- [go-arch-lint v3: every `deps` entry must declare at least one permission flag (`anyVendorDeps: true` for leaf components)](../../docs/backend/library-gotchas/go-arch-lint-v3-anyvendordeps-required.md)
- [go-arch-lint `commonComponents` is the only universal-import mechanism — cross-cutting packages still need explicit `mayDependOn`](../../docs/backend/library-gotchas/go-arch-lint-commoncomponents-not-universal.md)
- [go-arch-lint violation output format and import-graph scope (binary vs. archfile version; what AST shapes it cannot enforce)](../../docs/backend/library-gotchas/go-arch-lint-violation-output-and-scope.md)
- [go-arch-lint: exclude testdata Go files and regenerate generated packages before checking](../../docs/backend/library-gotchas/go-arch-lint-testdata-and-generated-exclusions.md)
- [`go get -tool`: installing Go tools as module dependencies (Go 1.24+)](../../docs/backend/library-gotchas/go-mod-tool-directive.md)
