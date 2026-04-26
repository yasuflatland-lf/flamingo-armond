# Render module

Provisions the Go / Echo backend service. Build/start/health values match the
Go module layout under `backend/` (`cmd/server/main.go` is the entrypoint).

## Inputs

| Name | Description |
|---|---|
| `service_name` | Render service name. |
| `github_repo` | `owner/name` slug. The Render GitHub App must already be installed. |
| `branch` | Tracked branch (default `main`). |
| `region`, `plan` | Render region and plan tier. Validated for shape. |
| `supabase_db_url`, `supabase_jwks_url`, `supabase_jwt_issuer`, `supabase_jwt_audience` | Wired from the supabase module. |
| `otel_endpoint` | Optional OTLP HTTP collector URL. Empty disables tracing. |

## Outputs

| Name | Description |
|---|---|
| `service_id` | Render service ID. |
| `service_url` | Public URL — feed to the vercel module as `backend_url`. |

## Notes

- `auto_deploy = false`: deploys are triggered explicitly so migration runs
  stay tied to intentional deploy events. The first deploy and every subsequent
  one is operator- or CI-driven (the deploy hook in `.github/workflows/backend.yml`).
- `GRAPHQL_INTROSPECTION=off`, `APP_ENV=production`, and
  `OTEL_TRACES_SAMPLER_ARG=0.1` are fixed in the module body. Override at the
  module level if a future env needs different values.
- `lifecycle { prevent_destroy = true }` matches the guard on
  `supabase_project`. A change to `region` or `owner_id` would force resource
  replacement; the guard requires deliberate operator action to opt in.

## Provider schema gotchas

Two attributes the `render-oss/render` provider places **outside** the
documentation's example shape. Caught by `terraform validate` in CI; both fail
with cryptic "unsupported argument" / "must be specified when …" errors:

- `owner_id` is configured **on the provider**, not on `render_web_service`.
  Setting it on the resource fails with `An argument named "owner_id" is not
  expected here.`
- `start_command` sits at the **resource top level**, not nested inside
  `runtime_source.native_runtime`. `build_command` is nested; `start_command`
  is not. Mixing them up fails with `Attribute "start_command" must be
  specified when "runtime_source.native_runtime" is specified` — Terraform
  thinks `start_command` is missing even when it's set inside the nested
  block.

Keep both pinned in the resource exactly where they are today.
