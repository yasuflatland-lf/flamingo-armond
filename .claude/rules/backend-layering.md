# Backend layering

> Applies to: `backend/`. Source of truth: [`backend/.go-arch-lint.yml`](../../backend/.go-arch-lint.yml). Cross-cutting because every package in `backend/internal/` and `backend/cmd/` participates in the layer graph.

## The invariants

The five invariants the config encodes, with the failure mode each guards against. Test files (`*_test.go`) are excluded from enforcement via `excludeFiles` in [`backend/.go-arch-lint.yml`](../../backend/.go-arch-lint.yml) — tests legitimately cross layers (e.g. an integration test in `repository/` that calls a usecase helper). The excluded pattern applies to all components; there is no per-component override needed.

- **Domain has no inbound deps from infrastructure or transport; `domain/service` only depends on `domain` itself** (`internal/domain`, `internal/domain/service`). A domain package that imports infrastructure pulls business rules into a deployment detail and makes the domain untestable without a real database or HTTP stack.
- **Application (`internal/usecase`) does not import `internal/gqlerr`**. `gqlerr` is a wire-format constructor tied to the GraphQL transport. Importing it from the application layer couples business logic to the presentation protocol and creates an import cycle when the resolver calls back into usecase types. The `usecase` entry in `deps` deliberately omits `gqlerr` from `mayDependOn`; this prevents the application layer from acquiring a transport dependency that would close an import cycle with the resolver. The replacement path is: usecase code returns `ucerr.*` typed values; the resolver converts them to wire format via the single `gqlerr.FromUsecaseError` call site. See [`.claude/rules/error-wrapping.md` § "Legacy primitives (resolver-only)"](error-wrapping.md#legacy-primitives-resolver-only) for the per-gate rationale and the `ucerr` replacements.
- **Infrastructure (`internal/repository`, `internal/auth`, etc.) does not import `usecase` or the presentation layer**. Allowing adapter packages to call upward into application logic inverts the dependency arrow and makes it impossible to substitute adapters or test them in isolation.
- **Only the presentation layer and the composition root may import `gqlerr`** — concretely `graph/resolver`, `internal/handler/*`, `internal/middleware`, and `cmd/server`. Today `graph/resolver` and `cmd/server` are the only consumers of `gqlerr`; the YAML config's `mayDependOn` lists reflect that exactly. A new handler that needs to construct wire errors must update its component's `mayDependOn` to include `gqlerr` in the same change.
- **CLI entrypoints (`cmd/seed`, `cmd/schema-lint`) do not import the transport surface** (`gqlerr`, `resolver`). A seed or schema-lint binary that pulls in the HTTP/GraphQL stack becomes fragile and bloated; `anyVendorDeps: true` freezes that property and prevents drive-by import additions.

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

**Cross-cutting packages** (`logging`, `telemetry`, `cursor`) are intended for use across layers, but every importer must explicitly list them in `mayDependOn` — `commonComponents` (which would grant universal availability) is reserved for generated DTOs. This keeps the dependency graph explicit and prevents accidental coupling.

**`internal/usecase/ucerr`** is a shared-kernel sub-package that lives inside the application layer but has no `gqlerr` dependency. It provides the typed error constructors (`ucerr.NewValidationError`, `ucerr.NewForbiddenError`, `ucerr.ErrUnauthenticated`) that usecase code returns to the resolver. The resolver converts these to wire format via `gqlerr.FromUsecaseError`. See [`docs/backend/error-wrapping/alias-bridge-subpackage.md`](../../docs/backend/error-wrapping/alias-bridge-subpackage.md) for the cycle-safety analysis.

**`graph/generated` and `graph/model`** are gqlgen output. They are declared as `commonComponents` so every layer can reference the generated DTOs without listing them in every `mayDependOn` entry. Adding `graph/generated` or `graph/model` to a component's `mayDependOn` explicitly is redundant and should be avoided — commonComponents are implicitly available everywhere.

**`internal/gqlerr`** sits inside `internal/` but belongs to the Presentation layer — it is a wire-format constructor, not a domain or application primitive. The fact that it lives under `internal/` does not make it available to domain or application code; the `go-arch-lint` config enforces its actual layer membership.

**`internal/auth` lists `domain` in `mayDependOn`** — this is intentional and correct. `auth.Service.IsAdmin` compares a role membership query result against `domain.AdminRoleName`, a domain-owned constant that defines the canonical admin role name. Role membership is a domain concern; coupling the auth adapter to the canonical constant rather than a local string literal keeps the two in sync and prevents drift. The dependency arrow (`auth → domain`) is infrastructure importing domain, which is the normal inward direction.

**The CSRF posture is a transport/presentation-layer invariant.** The bearer-only credential rule (no cookie/session auth read in `internal/auth`), the explicit CORS allowlist requirement, and the JSON-only gqlgen POST transport together make the API structurally CSRF-immune. Adding CORS middleware or a cookie credential touches that contract — see [`docs/backend-auth.md` § "CSRF posture: bearer-only credential and CORS allowlist invariant"](../../docs/backend-auth.md#csrf-posture-bearer-only-credential-and-cors-allowlist-invariant).

## What `go-arch-lint` covers vs. doesn't

`go-arch-lint` operates at the **import-graph level only**. Even with `deepScan: true` it does not classify function-call, struct-literal, or string-literal shapes. The CI grep gates in [`.github/workflows/backend.yml`](../../.github/workflows/backend.yml) are evaluated against that capability:

| # | Step name in CI | Shape | Why grep, not go-arch-lint |
|---|---|---|---|
| 1 | Verify no `fmt.Errorf("%w")` remains | function-call + string arg | go-arch-lint sees imports only |
| 2 | Forbid `&ucerr.ValidationError` / `&ucerr.ForbiddenError` struct literals | composite literal | go-arch-lint sees imports only |
| 3 | Forbid resolver returning raw usecase error | call-sequence pattern | go-arch-lint sees imports only |
| 4 | Verify no hardcoded `extensions.code` literals outside `gqlerr` | string literal | go-arch-lint sees imports only |
| 5 | Schema-lint outcome-union enforcement | schema AST | schema-lint covers GraphQL AST |

The one import-shape gate that previously enforced "`usecase` must not import `gqlerr`" is now expressed in [`backend/.go-arch-lint.yml`](../../backend/.go-arch-lint.yml) — the `usecase` component deliberately omits `gqlerr` from its `mayDependOn` list.

## Follow-up: more aggressive gate retirement

Retiring the function-call, composite-literal, call-sequence, and string-literal gates requires a complementary tool. Two viable directions: (1) `golangci-lint` with `forbidigo` covers function-call shapes (regex over Go identifiers) and would retire the `fmt.Errorf("%w")` gate; (2) `ast-grep` covers composite-literal, call-sequence, and string-literal shapes declaratively and would retire the remaining three. `depguard` is not on this list — it overlaps with `go-arch-lint` (both are import-graph linters) and would not add coverage. Track adoption under a dedicated issue and link it from this section.

## Operating notes

**Run locally** from `backend/`:

```bash
cd backend
go tool go-arch-lint check --project-path .   # exits 1 on violations; output names the offending import
go tool go-arch-lint graph --out /tmp/arch.svg  # optional: render the dependency graph as SVG
```

The `go tool go-arch-lint` invocation works because the binary is declared as a tool dependency in `backend/go.mod` (a `tool` directive). No separate installation step is needed; `go mod download` fetches and checksum-verifies it as part of the normal module bootstrap. The tool's version is pinned in `go.sum` — update it via `go get -tool github.com/fe3dback/go-arch-lint@<new-tag> && go mod tidy`.

**Add a new component:**

1. Add a `components: { name: { in: path } }` entry in [`backend/.go-arch-lint.yml`](../../backend/.go-arch-lint.yml). The `in:` value is a path relative to `workdir` (`.`), so `internal/auth` not `backend/internal/auth`.
2. Add a corresponding `deps: { name: { mayDependOn: [...] } }` entry listing only the components this new package is allowed to import. Omitting the entry means the component is unchecked, which is never the right default. Use `anyVendorDeps: true` for components that depend only on stdlib and vendor packages; use `mayDependOn: [...]` for components that have project-internal deps.
3. If the component is generated code (gqlgen, goyacc output), consider adding it to `commonComponents` so other layers can reference it without repeating the entry in every `mayDependOn` list.
4. If the component is a composition root, use `anyProjectDeps: true`. If it is a CLI entrypoint that must stay self-contained, use `anyVendorDeps: true` — v3 spec validator requires every component to declare at least one permission flag (`mayDependOn`, `canUse`, `anyProjectDeps`, or `anyVendorDeps`).
5. Run `go tool go-arch-lint check --project-path .` locally before pushing — it exits 1 on violations and names the offending import. A failure means the new `mayDependOn` list is incomplete or the component already has a forbidden import in its current source tree.
6. Verify the new component does not introduce an import cycle: `go build ./...` will catch cycles that `go-arch-lint` does not model (a declared cycle in `mayDependOn` is not the same as the Go compiler's cycle detection).

Note: the config sets `ignoreNotFoundComponents: false`. This means a component name referenced in `mayDependOn` that does not appear in the `components:` block causes a hard error at check time. Add every referenced name to `components:` first; do not forward-reference a component that does not yet exist.

**Read violation output:**

A violation line looks like:

```
Component resolver shouldn't depend on backend/internal/gqlerr in /abs/path/graph/resolver/schema.resolvers.go:12
```

The two relevant facts are the component pair (`resolver` → `backend/internal/gqlerr`) and the offending file:line. Fix by either (a) removing the disallowed import from the named file, or (b) adjusting `mayDependOn` for the importing component in `backend/.go-arch-lint.yml` if the dependency is intentional. For machine-parseable output use `--output-type=json`; v1.15.0 nests entries under `.Payload.ArchWarningsDeps`.

## Background

`go-arch-lint` is preferred for import-graph enforcement over ad-hoc grep because: (1) it is declaration-first — the allowed graph is a versioned YAML file that doubles as architecture documentation; (2) it produces structured output (`--json`) that is easy to parse in CI and to surface in tooling; (3) it handles `commonComponents` and `anyProjectDeps` semantically, so composition roots and generated packages do not require boilerplate allowances in every component; (4) it integrates as a Go tool dependency (`go get -tool`) and is therefore checksum-verified and reproducible across environments without a separate install step.

The limitation that motivates keeping the remaining grep gates is fundamental to the approach: `go-arch-lint` reads the Go import graph, not the AST. It answers "does package A import package B?" but not "does file X call function Y?" or "does struct Z appear as a composite literal?". The function-call, composite-literal, call-sequence, and string-literal gates — plus the schema-lint gate — each enforce a shape below the import-graph level and cannot be retired without a complementary AST-aware tool.

See [`.claude/rules/error-wrapping.md`](error-wrapping.md) for the full wire-format and error-wrapping conventions that the import-graph invariants support.
