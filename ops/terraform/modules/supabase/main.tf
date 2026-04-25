# Supabase only surfaces the database password at project creation time. We
# generate it inside Terraform so the value is captured in state and can be
# rotated by tainting just this resource.
resource "random_password" "db" {
  length  = 32
  special = true
  # Pooler DSN is URL-encoded by the provider, but a few specials still cause
  # parser issues in some clients. Restrict to a safe alphabet.
  override_special = "!@#%^*-_=+"
}

resource "supabase_project" "this" {
  name              = var.project_name
  organization_id   = var.organization_id
  region            = var.region
  database_password = random_password.db.result

  lifecycle {
    # Recreating drops the database. Require explicit operator action to
    # remove this guard before any destroy.
    prevent_destroy = true
  }
}

# Auth + URL configuration. Kept as a separate resource so the dependency edge
# from vercel -> supabase_settings does not create a cycle with supabase_project.
resource "supabase_settings" "this" {
  project_ref = supabase_project.this.id

  auth = jsonencode({
    site_url                  = var.site_url
    uri_allow_list            = join(",", var.redirect_urls)
    external_google_enabled   = true
    external_google_client_id = var.google_oauth_client_id
    external_google_secret    = var.google_oauth_client_secret
  })
}

data "supabase_apikeys" "this" {
  project_ref = supabase_project.this.id
}

data "supabase_pooler" "this" {
  project_ref = supabase_project.this.id
}
