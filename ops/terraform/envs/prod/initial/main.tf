# Initial stack — creates the projects and services that rarely change.
#
# Dependencies (DAG-ordered):
#   module.supabase  ->  module.render
#   module.supabase  ->  module.vercel  (project_url, anon_key)
#   module.render    ->  module.vercel  (service_url for BACKEND_URL)

module "supabase" {
  source = "../../../modules/supabase"

  project_name    = local.supabase_project_name
  organization_id = var.supabase_organization_id
  region          = var.supabase_region
}

module "render" {
  source = "../../../modules/render"

  service_name = local.render_service_name
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
  source = "../../../modules/vercel"

  project_name               = local.vercel_project_name
  github_repo                = var.github_repo
  production_domain_override = var.vercel_production_domain_override

  backend_url       = module.render.service_url
  supabase_url      = module.supabase.project_url
  supabase_anon_key = module.supabase.anon_key
}
