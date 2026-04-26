# Operator-facing outputs ------------------------------------------------------
output "production_url" {
  description = "Live frontend URL."
  value       = module.vercel.production_url
}

output "backend_url" {
  description = "Render service URL the frontend proxies to."
  value       = module.render.service_url
  precondition {
    condition     = can(regex("^https://", module.render.service_url))
    error_message = "backend_url must be a non-empty https URL. The Render service may still be provisioning — wait for it to finish and re-apply."
  }
}

output "supabase_project_ref" {
  description = "Supabase project ref. Used to update the Google OAuth client redirect URI and read by the settings stack."
  value       = module.supabase.project_ref
  precondition {
    condition     = length(module.supabase.project_ref) > 0
    error_message = "supabase_project_ref must be non-empty before the settings stack can safely read it."
  }
}

output "supabase_project_url" {
  description = "Supabase base URL."
  value       = module.supabase.project_url
}

output "supabase_db_password" {
  description = "Generated DB password. Capture via `terraform output -raw supabase_db_password`."
  value       = module.supabase.db_password
  sensitive   = true
}

output "render_service_id" {
  description = "Render service ID. Used to trigger the first manual deploy via the Render API."
  value       = module.render.service_id
}

# Cross-stack output — consumed by ../settings via terraform_remote_state.
output "vercel_production_url" {
  description = "Full https URL of the Vercel production deployment. Read by the settings stack as site_url."
  value       = module.vercel.production_url
  precondition {
    condition     = can(regex("^https://", module.vercel.production_url))
    error_message = "vercel_production_url must be a non-empty https URL before the settings stack can safely read it."
  }
}
