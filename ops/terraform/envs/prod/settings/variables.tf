# Reused from the initial stack's mise.local.toml. Both stacks share the same
# Supabase access token so a single secret rotation covers both.
variable "supabase_access_token" {
  type      = string
  sensitive = true
}

# Google OAuth credentials are scoped to the settings stack only — they are not
# needed to create the Supabase project, only to wire the Auth provider on top
# of an already-created project.
variable "google_oauth_client_id" {
  description = "Google OAuth client ID for the Supabase Auth Google provider."
  type        = string
  validation {
    condition     = length(var.google_oauth_client_id) > 0
    error_message = "google_oauth_client_id must not be empty."
  }
}

variable "google_oauth_client_secret" {
  description = "Google OAuth client secret. Cannot be read back from Supabase once written."
  type        = string
  sensitive   = true
  validation {
    condition     = length(var.google_oauth_client_secret) > 0
    error_message = "google_oauth_client_secret must not be empty."
  }
}
