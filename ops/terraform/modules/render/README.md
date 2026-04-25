# Render module

Provisions the Go / Echo backend service. Replaces the legacy `render.yaml`
blueprint at the repo root — Terraform is the sole source of truth for the
Render service definition once this module is applied.

## Inputs

| Name | Description |
|---|---|
| `service_name` | Render service name. |
| `owner_id` | Render owner ID (team or user). |
| `github_repo` | `owner/name` slug. The Render GitHub App must already be installed. |
| `branch` | Tracked branch (default `main`). |
| `region`, `plan` | Render region and plan tier. |
| `supabase_db_url`, `supabase_jwks_url`, `supabase_jwt_issuer`, `supabase_jwt_audience` | Wired from the supabase module. |
| `otel_endpoint` | Optional OTLP HTTP collector URL. Empty disables tracing. |

## Outputs

| Name | Description |
|---|---|
| `service_id` | Render service ID. |
| `service_url` | Public URL — feed to the vercel module as `backend_url`. |

## Notes

- `auto_deploy = false` matches the previous `render.yaml` posture: deploys
  trigger manually so migration runs are explicit.
- `GRAPHQL_INTROSPECTION=off`, `APP_ENV=production`, and
  `OTEL_TRACES_SAMPLER_ARG=0.1` are fixed in the module body. Override at the
  module level if a future env needs different values.
