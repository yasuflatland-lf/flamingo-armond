# schema-lint

Enforces the outcome-union pattern for new GraphQL mutations that emit typed
`ucerr.*` errors. A mutation whose return type is a bare object type AND whose
usecase method emits `ucerr.NewValidationError`, `ucerr.NewForbiddenError`, or
`ucerr.ErrUnauthenticated` is a violation. Existing violations are frozen via
`allowlist.txt`; new mutations must use a `*Result` union from day one.

See [docs/backend/error-wrapping/outcome-union-enforcement.md](../../../docs/backend/error-wrapping/outcome-union-enforcement.md) for the full
promotion checklist and design rationale.

## Usage

Run from `backend/`:

```
go run ./cmd/schema-lint
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-mode` | `error` | `error`: exit 1 on violations or drift. `warn`: print but exit 0. |
| `-schema` | `../schema/*.graphql` | Path, glob, or comma-separated list for GraphQL SDL files. |
| `-resolver` | `graph/resolver/*.resolvers.go` | Path, glob, or comma-separated list for generated resolver files. |
| `-resolver-struct` | `graph/resolver/resolver.go` | Path to the `Resolver` struct declaration. |
| `-usecase` | `internal/usecase` | Path to the usecase package directory (non-recursive). |
| `-allowlist` | `cmd/schema-lint/allowlist.txt` | Path to the allowlist file. |

### Exit codes

| Code | Meaning |
|---|---|
| 0 | No violations and no drift (error mode), or `-mode=warn`. |
| 1 | At least one violation or schema/resolver drift in error mode. |
| 2 | Walker error: file not readable, schema parse error, or unknown `-mode`. |

## Layout

| File | Role |
|---|---|
| `schemawalk.go` | GraphQL SDL parser; classifies each mutation's return type as bare or not. |
| `resolverwalk.go` | Go AST walker for `*mutationResolver` methods across generated resolver files; extracts `r.<Field>.<Method>` calls. |
| `usecasewalk.go` | Go AST walker for usecase methods; 1-bit `EmitsTypedError` detection. |
| `classifier.go` | Joins the three walker outputs, applies the allowlist, reports violations and drifts. |
| `main.go` | CLI wiring; hardcoded `InterfaceToImpl` map for interface-typed usecase fields. |
| `allowlist.txt` | Frozen list of day-1 bare-emit mutations. |
| `testdata/` | Fixture inputs for walker unit tests. |

## Maintenance

**Adding a new `ucerr.*` constructor** — extend the `switch` in
`usecasewalk.go`'s `methodEmitsTypedError` function and add a fixture under
`testdata/` to cover it.

**Adding a new interface-typed usecase field to `Resolver`** — if the field
type is an interface (not a `*usecase.XUsecase` concrete pointer), add a
`"InterfaceName" -> "implName"` entry to the `interfaceToImpl` map in
`main.go`. Concrete pointer fields are resolved automatically from the struct.

**Allowlist rot** — when a mutation is promoted, the lint emits
`allowlist-rot: <mutation> is allowlisted but no longer violates; remove the entry`.
Delete the corresponding line from `allowlist.txt`.

**Helper-delegated emissions** — the walker inspects only direct method bodies,
not package-level helper functions called from those bodies. If a promoted
mutation's usecase delegates all `ucerr.*` calls to a helper, the lint will
not fire even without an allowlist entry. This is a known acceptable
false-negative; extend the walker with same-file helper inlining if a real
miss is detected.
