# Shared-kernel sub-package `usecase/ucerr` for typed application errors

> Part of the [error wrapping convention](../../../.claude/rules/error-wrapping.md) rules.

## Historical context

Before [#154](https://github.com/yasuflatland-lf/flamingo-armond/issues/154) the typed error types
(`ValidationError`, `ForbiddenError`, `ErrUnauthenticated`) lived in the `usecase` package.
`gqlerr` needed to import them for the conversion logic in `FromUsecaseError`, but `usecase` already
imported `gqlerr` for legacy transport-shape decisions — a cycle the Go compiler rejects.

The cycle-break was to extract the shared types into a **leaf sub-package** `backend/internal/usecase/ucerr`.
`ucerr` has no `gqlerr` dependency, so `gqlerr` can import it freely. A thin alias bridge (`usecase/errors.go`)
then re-exported the types under the parent name so existing callers saw no visible rename.

[#158](https://github.com/yasuflatland-lf/flamingo-armond/issues/158) removed the last legacy
`gqlerr.*` import from the `usecase` production files. With no cycle to protect, the alias bridge
was removed in [#160](https://github.com/yasuflatland-lf/flamingo-armond/issues/160).

## Current state: `ucerr` as a Shared Kernel

`backend/internal/usecase/ucerr` now functions as a [Shared Kernel](https://www.domainlanguage.com/ddd/reference/)
in the DDD sense: a small, stable package that multiple layers (`usecase`, `gqlerr`, resolvers, tests)
all import directly under its own name. There is **one canonical name per type** — `*ucerr.ValidationError`,
`*ucerr.ForbiddenError`, `ucerr.ErrUnauthenticated` — and no aliasing layer introduces a second name.

Key properties preserved:

- **Light compile graph.** `ucerr` imports only `errors` from the standard library. It has no GORM,
  repository, or transport dependencies, so any layer can import it without pulling in the full
  backend dependency tree.
- **`errors.Is` / `errors.As` identity.** Every layer uses the same type definition, so cross-layer
  `errors.Is` / `errors.As` checks work without special handling.
- **Single conversion site.** `gqlerr.FromUsecaseError` remains the only place where `ucerr` types
  are translated to `gqlerror.Error` wire format. Resolvers and tests do not duplicate that logic.

## Verification

Classification of `ucerr` types through `FromUsecaseError` is covered by tests in
`backend/internal/gqlerr/from_usecase_test.go` — `TestValidationErrorClassification` and
`TestErrUnauthenticatedClassification`.

## Verifying acyclicity at design time

Before adding a new dependency on `ucerr` from another package, confirm the import does not create
a cycle by running `go build ./...`, not by reading the import lists manually. A manual reading will
miss cycles introduced by transitively-imported files in large packages. See
[`.claude/rules/scope-discipline.md` § "Plan-document claims about the codebase's structure must be toolchain-verified"](../../../.claude/rules/scope-discipline.md#plan-document-claims-about-the-codebases-structure-must-be-toolchain-verified)
for the general rule and a smoke-build recipe.
