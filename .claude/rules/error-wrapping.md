# Error wrapping convention

> Applies to: `backend/internal/`, `backend/cmd/`. Enforced by CI (`.github/workflows/backend.yml`).

The backend uses [`github.com/rotisserie/eris`](https://github.com/rotisserie/eris) as the **only** error-wrapping library inside `backend/internal/` and `backend/cmd/`. The convention is:

| Situation | Use |
|---|---|
| New error at the originating call site | `eris.New("layer: short message")` |
| Wrapping an error at a layer boundary | `eris.Wrap(err, "layer: context")` |
| Wrapping with format args | `eris.Wrapf(err, "layer: %s", id)` |
| Validation error without an underlying cause | `eris.Errorf("layer: %s must be >= %d", field, min)` |
| Sentinel that other code matches via `errors.Is` | `errors.New("...")` (do **not** use eris) |

`fmt.Errorf("...: %w", err)` is **forbidden** in `backend/internal/` and `backend/cmd/`. CI fails the build if any such call sneaks back in (see `.github/workflows/backend.yml`).

## Sentinels

Sentinels used today: `repository.ErrNotFound`, and domain-level sentinels such as `domain.ErrCardgroupNameRequired` / `domain.ErrCardgroupNameTooLong`. New sentinels are allowed when (a) callers need to branch on identity, and (b) a string-equality match is fragile. Keep sentinels as plain `errors.New` so `errors.Is` works without going through eris's chain walk.

## Legacy primitives (resolver-only)

`gqlerr.BadUserInput` / `Unauthenticated` / `NewForbidden` / `Internal` / `Cancelled` are wire-format constructors. They MUST be called only from:

- The resolver layer (`backend/graph/resolver/`) — for resolver-internal errors such as auth pre-checks and DataLoader-nil guards.
- `gqlerr.FromUsecaseError` (the single conversion site that translates usecase typed errors to the wire format).

`backend/internal/usecase/` MUST NOT import `backend/internal/gqlerr`. New usecase code returns:

- `ucerr.ErrUnauthenticated` (sentinel) for unauthenticated paths.
- `&ucerr.ValidationError{Field, Message}` for field-level validation failures.
- `&ucerr.ForbiddenError{Message}` for authorization failures.
- `eris.Wrap(err, "usecase: <op>")` (or `eris.Errorf` / `eris.New`) for internal/chain errors.
- `context.Canceled` / `context.DeadlineExceeded` passed through (no wrapping).

The resolver wraps every usecase return error with `gqlerr.FromUsecaseError(ctx, err)` to translate to the wire form. Resolver-internal `gqlerr.*` calls remain unwrapped — they are already wire-format.

CI enforces this boundary: `gqlerr` imports in `backend/internal/usecase/` (non-test files) cause a hard error in `.github/workflows/backend.yml`.

## Sentinels — detailed cases (on-demand)

- [Sentinel layering: when to join with `errors.Join` and when to keep standalone](../../docs/backend/error-wrapping/sentinel-layering.md)
- [Postgres FK violation classification (`23503`)](../../docs/backend/error-wrapping/postgres-fk-violation-23503.md)
- [Postgres unique-violation classification (`23505`)](../../docs/backend/error-wrapping/postgres-unique-violation-23505.md)
- [Two-tier API pattern: open primitive + strict/typed wrapper (`gqlerr`, env-config)](../../docs/backend/error-wrapping/two-tier-api-pattern.md)

## Logging — detailed cases (on-demand)

- [Logging: `LogError` / `LogWarn` helpers](../../docs/backend/error-wrapping/logging-error-warn-helpers.md)
- [Defensive `eris.Wrap` at log sites that consume narrow interfaces](../../docs/backend/error-wrapping/defensive-eris-wrap-at-log-sites.md)
- [Test the `error_chain` shape, not just its presence](../../docs/backend/error-wrapping/test-error-chain-shape-not-presence.md)
- [Assert PII absence on log lines that carry `user_id`](../../docs/backend/error-wrapping/assert-pii-absence-on-log-lines.md)
- [Log a structured event when a batch item fails and earlier work will be dropped](../../docs/backend/error-wrapping/log-structured-event-when-batch-item-fails.md)

## Errors as data — detailed cases (on-demand)

- [Result Union: "errors as data" pattern — when to use `CreateCardResult`-style unions over `BadUserInputWithExtensions`](../../docs/backend/error-wrapping/result-union-errors-as-data.md)

## Conversion boundaries — detailed cases (on-demand)

- [`FromUsecaseError`: single conversion site for usecase → gqlerror](../../docs/backend/error-wrapping/from-usecase-error-conversion-site.md)
- [Pointer-receiver discipline for typed errors used with `errors.As`](../../docs/backend/error-wrapping/pointer-receiver-for-errors-as.md)
- [Alias-bridge sub-package for cycle-safe shared types (`usecase/ucerr`)](../../docs/backend/error-wrapping/alias-bridge-subpackage.md)
- [Typed classifier field over string-prefix matching at conversion boundaries](../../docs/backend/error-wrapping/typed-classifier-over-string-prefix.md)
- [Classifier check must run before any pipeline step that appends to the classified slice](../../docs/backend/error-wrapping/classifier-check-ordering-before-pipeline-mutation.md)

## Background

`eris` is preferred over the common alternatives because: `fmt.Errorf("%w")` carries no stack trace and only a string context; `pkg/errors` has had no upstream activity since its January 2020 v0.9.1 tag and lacks the structured JSON chain serialization this codebase relies on for the `error_chain` log attribute; `cockroachdb/errors` is heavier and pulls in many transitive deps, so revisit only when multi-service error portability or first-class Sentry SDK integration becomes a hard requirement; `joomcode/errorx` is focused on typed-error hierarchies, less aligned with our wrap-and-log need.

- [What `error_chain` looks like](../../docs/backend/error-wrapping/what-error-chain-looks-like.md)
