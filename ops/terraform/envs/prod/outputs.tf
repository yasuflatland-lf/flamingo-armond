output "production_url" {
  description = "Live frontend URL."
  value       = module.vercel.production_url
}

output "backend_url" {
  description = "Render service URL the frontend proxies to."
  value       = module.render.service_url
}

output "supabase_project_ref" {
  description = "Supabase project ref. Used to update the Google OAuth client redirect URI."
  value       = module.supabase.project_ref
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
