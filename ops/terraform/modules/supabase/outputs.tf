output "project_ref" {
  description = "Project reference (the slug used in https://<ref>.supabase.co URLs)."
  value       = supabase_project.this.id
}

output "project_url" {
  description = "Base URL of the Supabase project."
  value       = "https://${supabase_project.this.id}.supabase.co"
}

output "anon_key" {
  description = "Publishable / anon key. Public-safe but RLS-gated."
  value       = data.supabase_apikeys.this.anon_key
  sensitive   = true
}

output "db_url" {
  description = "Session-mode pooler DSN for golang-migrate compatibility."
  value       = data.supabase_pooler.this.url["session"]
  sensitive   = true
}

output "db_password" {
  description = "Generated DB password. Capture via `terraform output -raw` if needed."
  value       = random_password.db.result
  sensitive   = true
}

output "jwks_url" {
  description = "JWKS endpoint for backend JWT validation."
  value       = "https://${supabase_project.this.id}.supabase.co/auth/v1/.well-known/jwks.json"
}

output "jwt_issuer" {
  description = "Issuer claim value backend validates against."
  value       = "https://${supabase_project.this.id}.supabase.co/auth/v1"
}

output "jwt_audience" {
  description = "Audience claim value backend validates against."
  value       = "authenticated"
}
