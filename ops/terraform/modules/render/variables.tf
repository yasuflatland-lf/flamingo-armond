variable "service_name" {
  description = "Render service name."
  type        = string
}

variable "owner_id" {
  description = "Render owner ID (team or user). Returned by GET /v1/owners."
  type        = string
}

variable "github_repo" {
  description = "GitHub repo slug (owner/name). The Render GitHub App must already be installed on the repo."
  type        = string
}

variable "branch" {
  description = "Branch to track for deploys."
  type        = string
  default     = "main"
}

variable "region" {
  description = "Render region (e.g. singapore, oregon)."
  type        = string
}

variable "plan" {
  description = "Render service plan tier (free, starter, standard, etc)."
  type        = string
  default     = "free"
}

variable "supabase_db_url" {
  description = "Session-mode Postgres DSN."
  type        = string
  sensitive   = true
}

variable "supabase_jwks_url" {
  description = "JWKS endpoint for backend JWT validation."
  type        = string
}

variable "supabase_jwt_issuer" {
  description = "Expected JWT issuer claim."
  type        = string
}

variable "supabase_jwt_audience" {
  description = "Expected JWT audience claim."
  type        = string
}

variable "otel_endpoint" {
  description = "OTLP HTTP collector URL. Empty disables tracing (no-op exporter)."
  type        = string
  default     = ""
}
