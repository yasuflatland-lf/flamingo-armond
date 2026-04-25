# The dependency graph forms a single straight line:
#   module.supabase (project + db_password)
#     -> module.render  (consumes db_url, jwks, issuer, audience)
#     -> module.vercel  (consumes anon_key, project_url, render service_url)
#   and module.supabase (settings) consumes module.vercel.production_url.
#
# Splitting supabase_project and supabase_settings inside the supabase module
# keeps this single-apply: settings depend on the project and on vercel
# outputs, so Terraform's DAG places it last automatically.

module "supabase" {
  source = "../../modules/supabase"

  project_name               = local.supabase_project_name
  organization_id            = var.supabase_organization_id
  region                     = var.supabase_region
  google_oauth_client_id     = var.google_oauth_client_id
  google_oauth_client_secret = var.google_oauth_client_secret

  site_url      = module.vercel.production_url
  redirect_urls = ["${module.vercel.production_url}/auth/callback"]
}

module "render" {
  source = "../../modules/render"

  service_name = local.render_service_name
  owner_id     = var.render_owner_id
  github_repo  = var.github_repo
  region       = var.render_region
  plan         = var.render_plan

  supabase_db_url       = module.supabase.db_url
  supabase_jwks_url     = module.supabase.jwks_url
  supabase_jwt_issuer   = module.supabase.jwt_issuer
  supabase_jwt_audience = module.supabase.jwt_audience

  otel_endpoint = var.otel_endpoint
}

module "vercel" {
  source = "../../modules/vercel"

  project_name = local.vercel_project_name
  github_repo  = var.github_repo

  backend_url       = module.render.service_url
  supabase_url      = module.supabase.project_url
  supabase_anon_key = module.supabase.anon_key
}
