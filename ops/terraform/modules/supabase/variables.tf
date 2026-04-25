variable "project_name" {
  description = "Display name for the Supabase project."
  type        = string
}

variable "organization_id" {
  description = "Supabase organization ID that owns the project."
  type        = string
}

variable "region" {
  description = "Supabase region slug (e.g. ap-northeast-1)."
  type        = string
}

variable "google_oauth_client_id" {
  description = "Google OAuth client ID for the Supabase Auth Google provider."
  type        = string
}

variable "google_oauth_client_secret" {
  description = "Google OAuth client secret. Cannot be read back from Supabase once written."
  type        = string
  sensitive   = true
}

variable "site_url" {
  description = "Auth Site URL (typically the Vercel production domain). Wired in from the vercel module so the loopback step is part of the same apply."
  type        = string
}

variable "redirect_urls" {
  description = "List of allowed Auth redirect URLs. Should include the Vercel /auth/callback path."
  type        = list(string)
}
