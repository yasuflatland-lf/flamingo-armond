---
paths:
  - "backend/**"
---

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
- [Table-parameterized bulk helpers (`upsertManyTx`, `listFrontsByGroupTx`, `deleteByGroupAndFrontsTx`) — one implementation shared by `cards` and `master_cards` via `(tableName, fkColumn string)` and a domain-agnostic row struct](../../docs/backend/library-gotchas/table-parameterized-bulk-repo-helper.md)
- [citext unique columns: case-fold the in-memory dedup key before a multi-row ON CONFLICT upsert (else Postgres 21000); case-fold *every* lookup into that map; concentrate tests at a mirror's divergence point](../../docs/backend/library-gotchas/citext-dedup-before-multirow-upsert.md)
- [Consumer-defined narrow repository interface per usecase](../../docs/backend/library-gotchas/consumer-defined-narrow-repo-interface.md) — for the domain→service variant (same technique, architecture-enforced boundary) see [`docs/backend/ddd-patterns/consumer-defined-interface-cross-package.md`](../../docs/backend/ddd-patterns/consumer-defined-interface-cross-package.md)
- [Inject `*rand.Rand` into pure functions to keep tests deterministic](../../docs/backend/library-gotchas/inject-rand-rand-for-deterministic-test.md)
- [`newV7` indirection seam for rare-failure crypto helpers — package-private `var` swap enables failure-path chain-shape assertions](../../docs/backend/library-gotchas/newv7-test-seam.md)
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
- [Call-count error injection on a fake to cover the Nth call of a twice-called repo method](../../docs/backend/library-gotchas/call-count-error-injection-for-nth-call.md)
- [Optional feature: pass `nil` handler and let the router skip route registration](../../docs/backend/library-gotchas/optional-feature-nil-handler-skip-route.md)
- [goyacc lexer: recover by emitting NEWLINE instead of `0`, and use explicit skip productions for lone rows](../../docs/backend/library-gotchas/goyacc-lexer-recovery-via-newline.md)
- [gqlgen `transport.POST` response headers must be set at construction time](../../docs/backend/library-gotchas/gqlgen-transport-post-response-headers.md)
- [gqlgen wraps deleted-field resolvers in a `// !!! WARNING !!!` block — they are not auto-removed](../../docs/backend/library-gotchas/gqlgen-warning-block-on-deleted-resolver.md)
- [gqlgen `follow-schema` layout orphans the old resolver file when a schema FILE is renamed](../../docs/backend/library-gotchas/gqlgen-schema-file-rename-orphans-resolver.md)
- [gqlgen acronym enum: TYPE keeps the acronym (`CEFRLevel`), METHOD lowercases its tail (`CefrLevel`)](../../docs/backend/library-gotchas/gqlgen-acronym-enum-type-vs-method-casing.md)
- [gqlgen-managed resolver imports must match `gqlgen generate` output — a post-gen import reorder passes local build/vet/test but fails the CI regen-diff gate](../../docs/backend/library-gotchas/gqlgen-resolver-import-order-ci-gate.md)
- [Casing-asymmetric enum: persisted lowercase vs wire uppercase needs explicit mappers (exhaustive read switch + erroring write + round-trip test)](../../docs/backend/library-gotchas/casing-asymmetric-enum-mapper.md)
- [Panic value format: `%T %v` vs `%T`-only — PII trade-off](../../docs/backend/library-gotchas/panic-value-format-pii-tradeoff.md)
- [XOR-invariant outcome structs for mutually-exclusive results](../../docs/backend/library-gotchas/xor-invariant-outcome-struct.md)
- [Fire-and-forget goroutine: detach context from request lifecycle](../../docs/backend/library-gotchas/fire-and-forget-goroutine-detached-context.md)
- [Channel-based "never called" assertion via `select` + `time.After`](../../docs/backend/library-gotchas/channel-based-never-called-assertion.md)
- [Transactional read-modify-write must use the `tx` handle, not `r.db`](../../docs/backend/library-gotchas/gorm-tx-read-modify-write.md)
- [TOCTOU authorization guard: lock the read rows with `FOR UPDATE`, surface guard outcomes via a captured variable](../../docs/backend/library-gotchas/toctou-authorization-guard-for-update-lock.md)
- [Optimistic version concurrency for cross-request edits (vs. `FOR UPDATE` for in-tx guards)](../../docs/backend/library-gotchas/optimistic-version-concurrency-cross-request.md)
- [GORM `Updates(map) + RowsAffected + FindByID` for race-free single-field updates (replaces `Take + Save`)](../../docs/backend/library-gotchas/gorm-update-via-updates-rowsaffected-findbyid.md)
- [Int-typed domain enums need `IsValid()` on DB reconstitution](../../docs/backend/library-gotchas/gorm-enum-cast-isvalid.md)
- [GraphQL resolver: synthesized domain values must be deterministic](../../docs/backend/library-gotchas/graphql-resolver-stable-synthesis.md)
- [Raw SQL CLI: `sql.Open("pgx", dbURL)` + pgx stdlib, not GORM; validate DSN before Open; auth.users needs superuser DSN](../../docs/backend/library-gotchas/raw-sql-cli-pgx-stdlib.md)
- [`*sql.Rows`: explicit `rows.Close()` after loop in addition to `defer rows.Close()`](../../docs/backend/library-gotchas/sql-rows-explicit-close.md)
- [SQL comparison on a nullable LEFT JOIN column silently excludes NULL rows (three-valued logic) — deliberate, not a missing `IS NOT NULL`](../../docs/backend/library-gotchas/sql-null-comparison-excludes-unreviewed-rows.md)
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
- [Index every foreign key — a composite PK does not back its non-leading columns, so cascade deletes seq-scan](../../docs/backend-db.md#index-strategy)
- [`ON DELETE` FK action requires an integration test against a real database](../../docs/backend/library-gotchas/fk-action-integration-test.md)
- [Migration down/up roundtrip test: prove the reverse path preserves data](../../docs/backend/library-gotchas/migration-down-up-roundtrip-test.md)
- [Shared parallel test DB: isolate no-cursor `first=N` pagination queries via a search predicate or cursor, not `filterByIDs` alone](../../docs/backend/library-gotchas/shared-parallel-db-pagination-test-isolation.md)
- [RLS `INSERT-own` assertion on a PK-keyed table needs a fixture row that does not exist yet](../../docs/backend/library-gotchas/rls-insert-own-fresh-fixture-row.md)
- [LSP stale-cache diagnostics during refactor — verify with `go build` before treating as real](../../docs/backend/library-gotchas/lsp-stale-cache-during-refactor.md)
- [CI bash `rc=$?` is dead code under `set -e` — use `cmd || rc=$?`](../../docs/backend/library-gotchas/ci-bash-rc-capture-under-set-e.md)
- [Walker / parser positive-discovery guards — silent zero disables enforcement](../../docs/backend/library-gotchas/walker-parser-positive-discovery-guards.md)
- [Resolver-injected usecase: interface field unlocks unit tests for outcome-union guards](../../docs/backend/library-gotchas/resolver-usecase-interface-vs-concrete-testability.md)
- [Usecase interface promotion: three-part template, same-package type assertions, `WithTx` refactoring, pointer-to-interface anti-pattern](../../docs/backend/library-gotchas/usecase-interface-promotion-pattern.md)
- [go-arch-lint v3: every `deps` entry must declare at least one permission flag (`anyVendorDeps: true` for leaf components)](../../docs/backend/library-gotchas/go-arch-lint-v3-anyvendordeps-required.md)
- [go-arch-lint `commonComponents` is the only universal-import mechanism — cross-cutting packages still need explicit `mayDependOn`](../../docs/backend/library-gotchas/go-arch-lint-commoncomponents-not-universal.md)
- [go-arch-lint violation output format and import-graph scope (binary vs. archfile version; what AST shapes it cannot enforce)](../../docs/backend/library-gotchas/go-arch-lint-violation-output-and-scope.md)
- [go-arch-lint: exclude testdata Go files and regenerate generated packages before checking](../../docs/backend/library-gotchas/go-arch-lint-testdata-and-generated-exclusions.md)
- [go-arch-lint deepScan attributes concrete-type value-flow to the producing component — widen to the port interface at the wiring site](../../docs/backend/library-gotchas/go-arch-lint-deepscan-concrete-type-value-flow.md)
- [`go get -tool`: installing Go tools as module dependencies (Go 1.24+)](../../docs/backend/library-gotchas/go-mod-tool-directive.md)
- [Bool flag vs two-function split: ubiquitous language signals (`authorizeCardgroup*`)](../../docs/backend/library-gotchas/bool-flag-vs-two-function-ubiquitous-language.md)
- [Direct unit tests for shared helpers + directionality assertion](../../docs/backend/library-gotchas/direct-unit-test-for-shared-helper.md)
- [Mock snapshot at call time for "X before Y" ordering assertions](../../docs/backend/library-gotchas/mock-snapshot-for-ordering-assertion.md)
- [Dead context-done branch in pass-through helper — collapse to single return](../../docs/backend/library-gotchas/dead-context-check-in-pass-through-helper.md)
- [Dead helper pipeline after an upstream gate — collapse to single wrap](../../docs/backend/library-gotchas/dead-pipeline-after-upstream-gate-collapse.md)
- [Preview and commit must share one validator (single-source validation parity); check before dedup so both consumers see identical input; whole-payload reject short-circuits per-row errors](../../docs/backend/library-gotchas/preview-commit-validation-parity.md)
- [GORM v1 round-trips underlying-string newtypes without Scanner/Valuer; reserving the `Value` accessor slot](../../docs/backend/library-gotchas/gorm-newtype-string-no-scanner-valuer.md)
- [Defensive-copy tests assert pointer identity (`require.NotSame`), not variable rebind](../../docs/backend/library-gotchas/defensive-copy-test-pointer-identity.md)
- [Mock-pointer fixture pitfalls: directionality tautology and parallel sub-test races](../../docs/backend/library-gotchas/mock-pointer-fixture-pitfalls.md)
- [`require.Equal` silently fails on typed string newtypes — `go build`/`go vet` miss it; only running the suite surfaces it](../../docs/backend/library-gotchas/testify-equal-typed-newtype-boxing.md)
- [GORM embedded struct with `TableName()` silently breaks the outer scan target](../../docs/backend/library-gotchas/gorm-embedded-tablename-scan-confusion.md)
- [Interleave trailing-append paths need a non-divisible fixture per direction](../../docs/backend/library-gotchas/interleave-trailing-append-test-fixture.md)
- [Exact-boundary fixture for strict time-cutoff predicates, proven by temporary mutation](../../docs/backend/library-gotchas/strict-cutoff-boundary-fixture-and-mutation-proof.md)
- [Fat repository interface split: CRUD vs membership seam — when and how to split, wiring checklist, context pass-through in validation helpers](../../docs/backend/library-gotchas/fat-repository-interface-split.md)
- [Codegen two-pass failure mode: schema field drop blocks regen until production code is fixed](../../docs/backend/library-gotchas/codegen-two-pass-schema-field-drop.md)
- [CI bash glob `for $(ls)` silently passes on zero matches under `set -e` — use `nullglob` array form](../../docs/backend/library-gotchas/ci-bash-glob-nullglob-silent-pass.md)
- [Inject `AdminChecker` (bool) for admin-exempt business rules, not `AdminGate.Require`](../../docs/backend/library-gotchas/admin-checker-inject-for-admin-exempt-business-logic.md)
- [`swipeUsecase` deliberately bypasses `Clock`, and its replay guard is learn-day granular](../../docs/backend/library-gotchas/swipe-bypasses-clock-port-and-day-granular-replay-guard.md)
- [go-fsrs v4's two elapsed-day clocks](../../docs/backend/library-gotchas/go-fsrs-v4-elapsed-day-clocks.md)

## DDD patterns (on-demand)

For domain-layer design patterns — value object constructors, aggregate behaviour
methods, trinary VOs, exported bound constants, and speculative-helper deletion —
see [`.claude/rules/ddd-patterns.md`](ddd-patterns.md) and the full chapter
index under `docs/backend/ddd-patterns/`.
