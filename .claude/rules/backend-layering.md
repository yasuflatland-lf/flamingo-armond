# Backend layering

> Applies to: `backend/`. Source of truth: [`backend/.go-arch-lint.yml`](../../backend/.go-arch-lint.yml). Cross-cutting because every package in `backend/internal/` and `backend/cmd/` participates in the layer graph.

## The invariants

The five invariants the config encodes, with the failure mode each guards against. Test files (`*_test.go`) are excluded from enforcement via `excludeFiles` in [`backend/.go-arch-lint.yml`](../../backend/.go-arch-lint.yml) — tests legitimately cross layers (e.g. an integration test in `repository/` that calls a usecase helper). The excluded pattern applies to all components; there is no per-component override needed.

- **Domain depends on nothing in `backend/internal/`** (`internal/domain`, `internal/domain/service`). A domain package that imports infrastructure pulls business rules into a deployment detail and makes the domain untestable without a real database or HTTP stack.
- **Application (`internal/usecase`) does not import `internal/gqlerr`**. `gqlerr` is a wire-format constructor tied to the GraphQL transport. Importing it from the application layer couples business logic to the presentation protocol and creates an import cycle when the resolver calls back into usecase types. The `usecase` entry in `deps` deliberately omits `gqlerr` from `mayDependOn` — this is the invariant a previous walk-back broke and the reason the machine check now enforces it. The replacement path is: usecase code returns `ucerr.*` typed values; the resolver converts them to wire format via the single `gqlerr.FromUsecaseError` call site. See [`.claude/rules/error-wrapping.md` § "Legacy primitives (resolver-only)"](error-wrapping.md#legacy-primitives-resolver-only) for the per-gate rationale and the `ucerr` replacements.
- **Infrastructure (`internal/repository`, `internal/auth`, etc.) does not import `usecase` or the presentation layer**. Allowing adapter packages to call upward into application logic inverts the dependency arrow and makes it impossible to substitute adapters or test them in isolation.
- **Only the presentation layer and the composition root may import `gqlerr`** — concretely `graph/resolver`, `internal/handler/*`, `internal/middleware`, and `cmd/server`. Today only `graph/resolver` and `cmd/server` use it; the broader allowance keeps future handler-side use legal without a config change.
- **CLI entrypoints (`cmd/seed`, `cmd/schema-lint`) do not import the transport surface** (`gqlerr`, `resolver`). A seed or schema-lint binary that pulls in the HTTP/GraphQL stack becomes fragile and bloated; `mayDependOn: []` freezes that property and prevents drive-by import additions.

## Layer model

The backend follows a DDD/layered architecture. The dependency arrow always points inward — outer layers import inner layers, never the reverse:

| Layer | Packages |
|---|---|
| Domain (core) | `internal/domain`, `internal/domain/service` |
| Application | `internal/usecase`, `internal/usecase/ucerr` |
| Infrastructure (adapter) | `internal/repository`, `internal/auth`, `internal/notion`, `internal/textdic`, `internal/database`, `internal/loader` |
| Presentation (transport) | `graph/resolver`, `internal/handler/notionsync`, `internal/handler/ping`, `internal/middleware`, `internal/gqlerr` |
| Composition root | `cmd/server` |
| Standalone CLI | `cmd/seed`, `cmd/schema-lint` |
| Cross-cutting | `internal/logging`, `internal/telemetry`, `internal/cursor` |
| Generated (common) | `graph/generated`, `graph/model` |

**Cross-cutting packages** (`logging`, `telemetry`, `cursor`) have no inbound restrictions and may be imported by any layer. They are listed as components with their own `mayDependOn` entries to prevent them from accidentally acquiring internal dependencies that would create cycles. `cursor` is a pure utility package; `logging` and `telemetry` may depend only on each other's stable interfaces, not on domain or usecase types.

**`internal/usecase/ucerr`** is a shared-kernel sub-package that lives inside the application layer but has no `gqlerr` dependency. It provides the typed error constructors (`ucerr.NewValidationError`, `ucerr.NewForbiddenError`, `ucerr.ErrUnauthenticated`) that usecase code returns to the resolver. The resolver converts these to wire format via `gqlerr.FromUsecaseError`. See [`docs/backend/error-wrapping/alias-bridge-subpackage.md`](../../docs/backend/error-wrapping/alias-bridge-subpackage.md) for the cycle-safety analysis.

**`graph/generated` and `graph/model`** are gqlgen output. They are declared as `commonComponents` so every layer can reference the generated DTOs without listing them in every `mayDependOn` entry. Adding `graph/generated` or `graph/model` to a component's `mayDependOn` explicitly is redundant and should be avoided — commonComponents are implicitly available everywhere.

**`internal/gqlerr`** sits inside `internal/` but belongs to the Presentation layer — it is a wire-format constructor, not a domain or application primitive. The fact that it lives under `internal/` does not make it available to domain or application code; the `go-arch-lint` config enforces its actual layer membership.

## What `go-arch-lint` covers vs. doesn't

`go-arch-lint` operates at the **import-graph level only**. Even with `deepScan: true` it does not classify function-call, struct-literal, or string-literal shapes. The six existing CI grep gates in [`.github/workflows/backend.yml`](../../.github/workflows/backend.yml) are evaluated against that capability:

| # | Step name in CI | Shape | Coverable by `go-arch-lint`? | Decision |
|---|---|---|---|---|
| 1 | Verify no `fmt.Errorf("%w")` remains | function-call + string arg | No | **keep** |
| 2 | Forbid `gqlerr` imports inside `usecase` | import shape | **Yes** | **delete** (replaced by `go-arch-lint`) |
| 3 | Forbid `&ucerr.ValidationError` / `&ucerr.ForbiddenError` struct literals | composite literal | No | **keep** |
| 4 | Forbid resolver returning raw usecase error | call-sequence pattern | No | **keep** |
| 5 | Verify no hardcoded `extensions.code` literals outside `gqlerr` | string literal | No | **keep** |
| 6 | Schema-lint outcome-union enforcement | schema AST | No | **keep** |

Only gate #2 is removable today. Gates #1, #3, #4, #5, and #6 must stay as grep- or AST-based checks because `go-arch-lint` has no visibility into sub-import-level shapes.

## Follow-up: more aggressive gate retirement

Retiring gates #1, #3, and #5 requires a complementary tool. Two viable directions: (1) `golangci-lint` with `depguard` + `forbidigo` covers function-call and string-literal shapes via configuration — this would retire the `fmt.Errorf("%w")` ban (#1), the struct-literal ban (#3), and the hardcoded `extensions.code` string-literal ban (#5); (2) `ast-grep` covers composite-literal and call-sequence shapes declaratively and would also address the resolver-wrap call-sequence gate (#4). Neither is bundled into the current change — `golangci-lint` adoption is explicitly out of scope here. A follow-up issue should be opened for "adopt `golangci-lint` + `depguard`/`forbidigo` to retire grep gates #1/#3/#5" and cross-linked here once it exists.

## Operating notes

**Run locally** from `backend/`:

```bash
cd backend
go tool go-arch-lint check --project-path .
go tool go-arch-lint check --project-path . --json | jq '.violations | length'   # expect 0
go tool go-arch-lint graph --out /tmp/arch.svg                                    # optional: render the dependency graph as SVG
```

The `go tool go-arch-lint` invocation works because the binary is declared as a tool dependency in `backend/go.mod` (a `tool` directive). No separate installation step is needed; `go mod download` fetches and checksum-verifies it as part of the normal module bootstrap. The tool's version is pinned in `go.sum` — update it via `go get -tool github.com/fe3dback/go-arch-lint@<new-tag> && go mod tidy`.

**Add a new component:**

1. Add a `components: { name: { in: path } }` entry in [`backend/.go-arch-lint.yml`](../../backend/.go-arch-lint.yml). The `in:` value is a path relative to `workdir` (`.`), so `internal/auth` not `backend/internal/auth`.
2. Add a corresponding `deps: { name: { mayDependOn: [...] } }` entry listing only the components this new package is allowed to import. Omitting the entry is not the same as an empty `mayDependOn: []` — an absent entry means the component is unchecked, which is never the right default.
3. If the component is generated code (gqlgen, goyacc output), consider adding it to `commonComponents` so other layers can reference it without repeating the entry in every `mayDependOn` list.
4. If the component is a composition root, use `anyProjectDeps: true`. If it is a CLI entrypoint that must stay self-contained, use `mayDependOn: []`.
5. Run `go tool go-arch-lint check --project-path . --json | jq '.violations | length'` locally before pushing. A non-zero count means the new `mayDependOn` list is incomplete or the component already has a forbidden import in its current source tree.
6. Verify the new component does not introduce an import cycle: `go build ./...` will catch cycles that `go-arch-lint` does not model (a declared cycle in `mayDependOn` is not the same as the Go compiler's cycle detection).

Note: the config sets `ignoreNotFoundComponents: false`. This means a component name referenced in `mayDependOn` that does not appear in the `components:` block causes a hard error at check time. Add every referenced name to `components:` first; do not forward-reference a component that does not yet exist.

**Read violation output:**

A violation line looks like:

```
internal/usecase/card.go:14:2: component "usecase" cannot import "gqlerr" (not in allowedList)
```

The format is `file:line:column: component "<A>" cannot import "<B>" (not in allowedList)`. Fix by either (a) removing the import from the source file if it violates the intended layer boundary, or (b) adding `B` to `deps.A.mayDependOn` in the config after confirming the dependency is intentional and does not close a cycle. Do not add allowances silently — the config is the machine record of the intended layer graph; loosening it without justification defeats its purpose. When in doubt, verify with `go list -deps ./internal/usecase/...` to see the full transitive set before editing `mayDependOn`.

## Background

`go-arch-lint` is preferred for import-graph enforcement over ad-hoc grep because: (1) it is declaration-first — the allowed graph is a versioned YAML file that doubles as architecture documentation; (2) it produces structured output (`--json`) that is easy to parse in CI and to surface in tooling; (3) it handles `commonComponents` and `anyProjectDeps` semantically, so composition roots and generated packages do not require boilerplate allowances in every component; (4) it integrates as a Go tool dependency (`go get -tool`) and is therefore checksum-verified and reproducible across environments without a separate install step.

The limitation that motivates keeping the remaining grep gates is fundamental to the approach: `go-arch-lint` reads the Go import graph, not the AST. It answers "does package A import package B?" but not "does file X call function Y?" or "does struct Z appear as a composite literal?". The grep gates (#1, #3, #4, #5) and the schema-lint gate (#6) each enforce a shape below the import-graph level and cannot be retired without a complementary AST-aware tool.

See [`.claude/rules/error-wrapping.md`](error-wrapping.md) for the full wire-format and error-wrapping conventions that the import-graph invariants support.
