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

## Sentinels — detailed cases (on-demand)

- [Layered sentinels via `errors.Join`](../../docs/backend/error-wrapping/layered-sentinels-via-errors-join.md)
- [Postgres FK violation classification (`23503`)](../../docs/backend/error-wrapping/postgres-fk-violation-23503.md)
- [Postgres unique-violation classification (`23505`)](../../docs/backend/error-wrapping/postgres-unique-violation-23505.md)
- [Standalone sentinels: not every new sentinel joins `ErrNotFound`](../../docs/backend/error-wrapping/standalone-sentinels-not-every-joins-errnotfound.md)
- [Two-tier `gqlerr` API: generic open helper + domain-specific typed wrapper](../../docs/backend/error-wrapping/two-tier-gqlerr-api.md)
- [Two-tier env config API: optional reader + strict wrapper](../../docs/backend/error-wrapping/two-tier-optional-strict-env-config.md)

## Logging — detailed cases (on-demand)

- [Logging: `LogError` / `LogWarn` helpers](../../docs/backend/error-wrapping/logging-error-warn-helpers.md)
- [Defensive `eris.Wrap` at log sites that consume narrow interfaces](../../docs/backend/error-wrapping/defensive-eris-wrap-at-log-sites.md)
- [Test the `error_chain` shape, not just its presence](../../docs/backend/error-wrapping/test-error-chain-shape-not-presence.md)
- [Assert PII absence on log lines that carry `user_id`](../../docs/backend/error-wrapping/assert-pii-absence-on-log-lines.md)
- [Parser-derived error messages must not leak into log fields — line + count only](../../docs/backend/error-wrapping/parser-derived-log-fields-pii-risk.md)

## Background (on-demand)

- [Why `eris` over alternatives](../../docs/backend/error-wrapping/why-eris-over-alternatives.md)
- [What `error_chain` looks like](../../docs/backend/error-wrapping/what-error-chain-looks-like.md)
