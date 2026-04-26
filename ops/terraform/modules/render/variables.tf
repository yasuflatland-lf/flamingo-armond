variable "service_name" {
  description = "Render service name."
  type        = string
  validation {
    condition     = length(var.service_name) > 0
    error_message = "service_name must not be empty."
  }
}

variable "github_repo" {
  description = "GitHub repo slug (owner/name). The Render GitHub App must already be installed on the repo."
  type        = string
  validation {
    condition     = can(regex("^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$", var.github_repo))
    error_message = "github_repo must be in the form owner/name."
  }
}

variable "branch" {
  description = "Branch to track for deploys."
  type        = string
  default     = "main"
}

variable "region" {
  description = "Render region. Update the validation list when Render adds a new region."
  type        = string
  validation {
    condition = contains(
      ["singapore", "oregon", "ohio", "virginia", "frankfurt"],
      var.region,
    )
    error_message = "region must be one of singapore, oregon, ohio, virginia, frankfurt. Update this list when Render announces a new region."
  }
}

variable "plan" {
  description = "Render service plan tier."
  type        = string
  default     = "free"
  validation {
    condition = contains(
      ["free", "starter", "standard", "pro", "pro_plus", "pro_max", "pro_ultra"],
      var.plan,
    )
    error_message = "plan must be one of free, starter, standard, pro, pro_plus, pro_max, pro_ultra. Update the validation list when Render adds a new tier."
  }
}

variable "supabase_db_url" {
  description = "Session-mode Postgres DSN."
  type        = string
  sensitive   = true
  validation {
    # Pooler DSN format: postgres://...:...@...:6543/postgres
    condition     = can(regex("^postgres(ql)?://", var.supabase_db_url))
    error_message = "supabase_db_url must be a postgres:// or postgresql:// DSN."
  }
}

variable "supabase_jwks_url" {
  description = "JWKS endpoint for backend JWT validation."
  type        = string
  validation {
    condition     = can(regex("^https://", var.supabase_jwks_url))
    error_message = "supabase_jwks_url must be an https URL."
  }
}

variable "supabase_jwt_issuer" {
  description = "Expected JWT issuer claim."
  type        = string
  validation {
    condition     = can(regex("^https://", var.supabase_jwt_issuer))
    error_message = "supabase_jwt_issuer must be an https URL."
  }
}

variable "supabase_jwt_audience" {
  description = "Expected JWT audience claim."
  type        = string
  validation {
    condition     = length(var.supabase_jwt_audience) > 0
    error_message = "supabase_jwt_audience must not be empty."
  }
}

variable "otel_endpoint" {
  description = "OTLP HTTP collector URL. Empty disables tracing (no-op exporter)."
  type        = string
  default     = ""
  validation {
    condition     = var.otel_endpoint == "" || can(regex("^https?://", var.otel_endpoint))
    error_message = "otel_endpoint must be empty or an http(s) URL."
  }
}
