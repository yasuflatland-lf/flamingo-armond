# Outcome-union enforcement lint

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.
> Closely related to [`docs/backend/error-wrapping/result-union-errors-as-data.md`](result-union-errors-as-data.md).

## Overview

The schema-lint program at `backend/cmd/schema-lint/` detects mutations that
return a bare object type while their backing usecase method emits at least one
typed `ucerr.*` variant. Such mutations are promotion candidates for the
outcome-union pattern described in
[`docs/backend/error-wrapping/result-union-errors-as-data.md`](result-union-errors-as-data.md).

"Bare" means the mutation's return type is a single named object or interface —
not wrapped in a `union`. A bare return type forces the resolver to route
typed business failures through the `error` channel, where they lose schema
structure and reach the client as opaque `extensions.code` strings. Wrapping
the return type in a union instead lets codegen produce typed variants the
client can switch on safely.

The lint runs in `.github/workflows/backend.yml` alongside the existing
error-handling gates (see [`.claude/rules/error-wrapping.md`](../../../.claude/rules/error-wrapping.md)).
It is a stop-the-bleeding gate: existing mutations that predate the rule are
frozen in an allowlist so the gate's introduction does not blast-radius across
the codebase. New mutations that match both conditions fail immediately.

## What the lint flags

The classifier joins three independent walkers:

- `schemawalk` — reads `schema/schema.graphql` via `vektah/gqlparser`; produces
  `{mutation, returnType, isBare}` for every mutation field.
- `resolverwalk` — reads `backend/graph/resolver/schema.resolvers.go` as a Go
  AST; produces the mutation → usecase call mapping.
- `usecasewalk` — reads `backend/internal/usecase/*.go`; produces a 1-bit
  `emitsTypedError` flag per usecase method.

A violation is raised when `isBare AND emitsTypedError AND NOT in allowlist`.

### Return-type classification (`schemawalk`)

| Return type | Lint subject? | Reason |
|---|---|---|
| Object type (`Card!`, `User!`, `Role!`) | yes | Promotion candidate |
| Union (`CreateCardResult!`) | no | Already outcome-union |
| List (`[Card!]!`) | no | Bulk/list — different pattern |
| Scalar (`Boolean!`, `Int!`) | no | A success-only scalar variant (`type DeleteSuccess { deleted: Boolean }`) adds no value |
| Interface | yes | Defensive — does not occur today but logically equivalent to a bare object |

"Bare" = isolated object or interface return type that is not wrapped in a union.

### Typed-error emission detection (`usecasewalk`)

A usecase method is flagged `emitsTypedError = true` when its function body
contains any of:

- `ucerr.NewValidationError(...)` call expression
- `ucerr.NewForbiddenError(...)` call expression
- `ucerr.ErrUnauthenticated` identifier reference

Detection uses `ast.Inspect` on the function body only. Helper-delegated
emission outside the same function body is a known false-negative; its profile
is low today (verified via grep). Extend the walker if a future helper
extraction produces a real miss.

**Worked example of the helper-delegated false-negative.** `adminUserUsecase.AssignRole`
and `adminRoleUsecase.Create` were never on the allowlist before [#171] even though
both ultimately surfaced typed `ucerr.NewValidationError` values to the resolver.
The classifier missed them because both methods routed validation through helper
functions (`mapRoleAssignmentError`, `mapAdminRoleError`, `validateRoleName`)
that themselves called `ucerr.New*` — the helpers lived outside the methods'
function bodies, so the body-only `ast.Inspect` saw zero direct constructor
references and treated `emitsTypedError = false`. The promotion still landed
cleanly (no allowlist edit was required for either mutation), but a maintainer
relying on the allowlist as ground truth for "what is bare-emit today" would be
misled. When a future audit needs the true bare-emit set, grep the production
tree directly per
[`.claude/rules/pr-sizing.md` § "Re-verify call-site count before sizing"](../../../.claude/rules/pr-sizing.md#re-verify-call-site-count-before-sizing)
rather than treating the allowlist as exhaustive. The walker's positive-discovery
guards (see
[`docs/backend/library-gotchas/walker-parser-positive-discovery-guards.md`](../library-gotchas/walker-parser-positive-discovery-guards.md))
are independent — they catch missing inputs, not helper-delegated emission
in present-but-correctly-parsed source.

## Why allowlist (stop-the-bleeding philosophy)

The allowlist at `backend/cmd/schema-lint/allowlist.txt` follows the same
philosophy as `rubocop --auto-gen-config` and `mypy --baseline`: existing
violations are named and frozen at a known point; every new violation fails
immediately. This lets the gate ship in `-mode=error` on day one without
requiring a bulk promotion of the pre-existing bare-emit mutations frozen in
the allowlist.

Each promotion is one allowlist-line deletion paired with the full schema +
usecase + resolver + frontend + regenerate change. Because promotion PRs touch
exactly one mutation, they stay well inside the 800-line production-diff ceiling
in [`.claude/rules/pr-sizing.md`](../../../.claude/rules/pr-sizing.md).

Allowlist format: one mutation name per line. Lines starting with `#` and blank
lines are ignored. Inline `# reason` comments record why promotion is deferred.

```text
# backend/cmd/schema-lint/allowlist.txt
updateProfile          # NewValidationError("displayName"/"bio", ...): field-only payload
createCardgroup        # NewValidationError("name", ...): field-only payload
```

## Promotion checklist

Follow these steps in order to promote one mutation from the allowlist to a
full outcome-union. Each step is a single atomic unit; commit after step 4 and
after step 7 pass their respective verifications.

1. **Edit `schema/schema.graphql`**: add `TypeSuccess`, `TypeError1`,
   `TypeError2` types (error types implement `UserError` to gain `message:
   String!`), declare `union <Op>Result = TypeSuccess | TypeError1 | ...`, and
   change the mutation field's return type from the bare object to `<Op>Result!`.
   Docstring convention: dock the **union** with a one-line description of the
   outcomes ("Either the updated user or an input-validation failure."); leave
   the `<Op>Success` and per-variant error types bare. A per-Success-type
   docstring that restates the union docstring ("Success result of `foo`.
   Returned when foo succeeded.") adds nothing and accumulates as noise across
   the schema. Error types that need invariant-level context (e.g.
   `CannotRevokeOwnAdminRoleError` carries a docblock explaining the
   self-demotion rule) keep their docstrings — the rule is "narrate the
   non-obvious", not "no docstrings on variant types".

2. **Refactor the usecase**: introduce an `<Op>Outcome` struct with one pointer
   per variant; change the method signature to `(<Op>Outcome, error)`. The XOR
   invariant — exactly one non-nil pointer per returned outcome — is enforced by
   the exhaustiveness guard in the resolver (step 3). Domain-invariant violations
   return `(<Op>Outcome{VariantField: &info}, nil)`; all other failures remain in
   the `error` channel.

3. **Refactor the resolver**: branch on `outcome.<Variant> != nil` and return
   the corresponding `model.<Variant>` value; guard "no variant set" with
   `gqlerr.Internal(ctx, eris.New("resolver: <Op>Outcome has no variant set"))`.
   Wrap all usecase errors via `gqlerr.FromUsecaseError(ctx, err)` per the
   mandatory resolver-side wrap rule.

4. **Regenerate**: run `go tool gqlgen generate` from `backend/`; commit the
   resulting `backend/graph/generated/` and `backend/graph/model/` diff. Verify
   `go build ./...` and `go vet ./...` pass before continuing.

5. **Update the frontend mutation**: discriminate on `__typename` with inline
   fragments per variant in the `.graphql` document; regenerate types via
   `pnpm --filter frontend codegen`; commit the `frontend/src/generated/` diff
   and the updated mutation handler.

6. **Delete the mutation name from `backend/cmd/schema-lint/allowlist.txt`**:
   remove the line (and its inline comment) for the promoted mutation.

7. **Verify the lint exits cleanly**: run `go run ./cmd/schema-lint` from
   `backend/`; assert it exits 0 with no violations reported for the promoted
   mutation or any other.

## Maintaining the lint

### Naming drift handling

When gqlgen renames a resolver method (e.g. after a schema field rename),
`resolverwalk` can no longer pair the schema mutation with its resolver body.
The lint emits `schema/resolver drift: mutation <X> has no resolver method` and
fails with a non-zero exit code — independent of any outcome-union concern. This
surfaces gqlgen naming drift in CI on the first run after the change, before
any real violation analysis runs.

### Adding a new `ucerr.*` constructor

If a new constructor is added to `backend/internal/usecase/ucerr/` (e.g.
`ucerr.NewConflictError`), extend the constructor switch in
`methodEmitsTypedError` (`backend/cmd/schema-lint/usecasewalk.go`) with one
new case and add a fixture test in `backend/cmd/schema-lint/testdata/` to cover
the true and false cases. The fixture test runs without access to the
project-wide source tree; keep it self-contained.

### False-positive recovery

If the lint flags a mutation that should not be a violation (e.g. a resolver
that emits `ucerr.NewValidationError` only in an unreachable legacy path), add
the mutation name to `backend/cmd/schema-lint/allowlist.txt` with a `# reason`
comment explaining why. File a follow-up issue to investigate whether the
emission is intentional or a refactor leak; link the issue in the comment.
Never leave an unexplained allowlist entry.

### Allowlist rot detection is informational, not exit-code-affecting

When an allowlist entry no longer corresponds to a bare-emit mutation — because
the mutation was promoted to a union, deleted, renamed, or its usecase stopped
emitting typed errors — the lint prints `allowlist-rot: <mutation> is allowlisted
but no longer violates; remove from allowlist.txt` to stderr. **The rot check is
deliberately informational and does not affect the exit code.** A rotted entry
is harmless to enforcement (it forgives a violation that no longer exists), so
treating it as a hard CI failure would block unrelated PRs over a cleanup task.
Maintainers periodically delete rotted entries; the stderr noise is the prompt,
not a blocker. The implementation in `backend/cmd/schema-lint/main.go` prints
the rot warnings before the violation check and continues regardless. If a
future change wants to enforce zero rot in CI, that is a separate flag — do
not flip the existing behavior, because pre-existing rot blocks the next
unrelated PR until it is cleaned up.

## Back-links

- [`result-union-errors-as-data.md`](result-union-errors-as-data.md) — pattern reference; `createCard` is the canonical worked example; `updateRole`, `revokeRole`, `assignRole`, `adminUpdateUser`, and `createRole` are subsequent precedents that follow the same shape.
- [`input-validation-info-empty-field-panic.md`](input-validation-info-empty-field-panic.md) — construction invariant for the `InputValidationInfo` carrier shared across promoted outcomes.
- [`inverse-helper-for-partial-promotion.md`](inverse-helper-for-partial-promotion.md) — `lower*` / `lift*` pattern for sharing an error classifier between promoted and unpromoted callers in the same package.
- [`.claude/rules/error-wrapping.md` § "Errors as data — detailed cases"](../../../.claude/rules/error-wrapping.md#errors-as-data--detailed-cases-on-demand) — rule layer this doc supports.
- [`.claude/rules/scope-discipline.md`](../../../.claude/rules/scope-discipline.md) — allowlist-as-baseline rationale; per-mutation promotion as opportunistic follow-up.
