resource "render_web_service" "this" {
  # owner_id is configured on the provider (see envs/<env>/providers.tf), not
  # on the resource. The render-oss/render provider scopes resources to the
  # provider's owner_id, so passing it here would be a schema error.
  name           = var.service_name
  region         = var.region
  plan           = var.plan
  root_directory = "backend"
  # start_command sits at the resource top level by provider schema, even though
  # build_command lives nested inside runtime_source.native_runtime.
  start_command = "./main"

  runtime_source = {
    native_runtime = {
      auto_deploy   = false # Manual deploys keep migration runs explicit.
      branch        = var.branch
      build_command = "go mod download && go build -o main ./cmd/server"
      repo_url      = "https://github.com/${var.github_repo}"
      runtime       = "go"
    }
  }

  health_check_path = "/health"

  env_vars = {
    SUPABASE_DB_URL       = { value = var.supabase_db_url }
    SUPABASE_JWKS_URL     = { value = var.supabase_jwks_url }
    SUPABASE_JWT_ISSUER   = { value = var.supabase_jwt_issuer }
    SUPABASE_JWT_AUDIENCE = { value = var.supabase_jwt_audience }

    APP_ENV                 = { value = "production" }
    GRAPHQL_INTROSPECTION   = { value = "off" }
    OTEL_TRACES_SAMPLER_ARG = { value = "0.1" }

    OTEL_EXPORTER_OTLP_ENDPOINT = { value = var.otel_endpoint }
  }

  lifecycle {
    # A region or owner_id change forces replacement, taking the backend offline.
    # Remove this block deliberately before any destroy or forced-replace.
    prevent_destroy = true
  }
}
