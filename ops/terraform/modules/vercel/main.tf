resource "vercel_project" "this" {
  name           = var.project_name
  framework      = var.framework
  root_directory = var.root_directory

  git_repository = {
    type              = "github"
    repo              = var.github_repo
    production_branch = var.production_branch
  }
}

# Environment variables are declared per-key so each can be scoped and rotated
# independently. Targeting both production and preview keeps preview builds
# functional against the same backend; switch to ["production"] only if a
# distinct preview backend is introduced later.
resource "vercel_project_environment_variable" "backend_url" {
  project_id = vercel_project.this.id
  key        = "BACKEND_URL"
  value      = var.backend_url
  target     = ["production", "preview"]
}

resource "vercel_project_environment_variable" "supabase_url" {
  project_id = vercel_project.this.id
  key        = "NEXT_PUBLIC_SUPABASE_URL"
  value      = var.supabase_url
  target     = ["production", "preview", "development"]
}

resource "vercel_project_environment_variable" "supabase_anon_key" {
  project_id = vercel_project.this.id
  key        = "NEXT_PUBLIC_SUPABASE_ANON_KEY"
  value      = var.supabase_anon_key
  target     = ["production", "preview", "development"]
  sensitive  = true
}
