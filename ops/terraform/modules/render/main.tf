# Mirrors the contract previously declared in render.yaml. Build/start/health
# values are taken from cmd/server/main.go and the Go module layout under
# backend/.
resource "render_web_service" "this" {
  name           = var.service_name
  owner_id       = var.owner_id
  region         = var.region
  plan           = var.plan
  root_directory = "backend"

  runtime_source = {
    native_runtime = {
      auto_deploy   = false # Manual deploys keep migration runs explicit.
      branch        = var.branch
      build_command = "go mod download && go build -o main ./cmd/server"
      repo_url      = "https://github.com/${var.github_repo}"
      runtime       = "go"
      start_command = "./main"
    }
  }

  health_check_path = "/health"

  env_vars = {
    # --- Supabase (required at boot; backend fails fast if missing) ----------
    SUPABASE_DB_URL       = { value = var.supabase_db_url }
    SUPABASE_JWKS_URL     = { value = var.supabase_jwks_url }
    SUPABASE_JWT_ISSUER   = { value = var.supabase_jwt_issuer }
    SUPABASE_JWT_AUDIENCE = { value = var.supabase_jwt_audience }

    # --- App constants -------------------------------------------------------
    APP_ENV                 = { value = "production" }
    GRAPHQL_INTROSPECTION   = { value = "off" }
    OTEL_TRACES_SAMPLER_ARG = { value = "0.1" }

    # --- Optional tracing endpoint ------------------------------------------
    OTEL_EXPORTER_OTLP_ENDPOINT = { value = var.otel_endpoint }
  }
}
