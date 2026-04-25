# Common -----------------------------------------------------------------------
variable "project_name" {
  description = "Logical project name. Used as the base for service names across providers."
  type        = string
}

variable "github_repo" {
  description = "GitHub repo slug (owner/name)."
  type        = string
}

# Supabase ---------------------------------------------------------------------
variable "supabase_access_token" {
  type      = string
  sensitive = true
}

variable "supabase_organization_id" {
  type = string
}

variable "supabase_region" {
  type = string
}

variable "google_oauth_client_id" {
  type = string
}

variable "google_oauth_client_secret" {
  type      = string
  sensitive = true
}

# Render -----------------------------------------------------------------------
variable "render_api_key" {
  type      = string
  sensitive = true
}

variable "render_owner_id" {
  type = string
}

variable "render_region" {
  type = string
}

variable "render_plan" {
  type    = string
  default = "free"
}

variable "otel_endpoint" {
  description = "Optional OTLP HTTP collector URL."
  type        = string
  default     = ""
}

# Vercel -----------------------------------------------------------------------
variable "vercel_api_token" {
  type      = string
  sensitive = true
}

variable "vercel_team_id" {
  description = "Empty for personal account."
  type        = string
  default     = ""
}
