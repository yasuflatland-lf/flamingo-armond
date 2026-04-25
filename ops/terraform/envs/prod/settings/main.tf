# Settings stack — depends on the initial stack's outputs via local-state read.
#
# Why a separate stack?
#   * supabase_settings is the most-edited resource in the bring-up: any auth
#     change (redirect URLs, custom OAuth providers, custom domain) reapplies
#     here without touching DB password or project state.
#   * Initial stack apply remains rare. Settings stack apply can be frequent.

data "terraform_remote_state" "initial" {
  backend = "local"
  config = {
    path = "../initial/terraform.tfstate"
  }
}

locals {
  vercel_url = data.terraform_remote_state.initial.outputs.vercel_production_url
  redirect_urls = [
    "${local.vercel_url}/auth/callback",
  ]
}

resource "supabase_settings" "this" {
  project_ref = data.terraform_remote_state.initial.outputs.supabase_project_ref

  auth = jsonencode({
    site_url                  = local.vercel_url
    uri_allow_list            = join(",", local.redirect_urls)
    external_google_enabled   = true
    external_google_client_id = var.google_oauth_client_id
    external_google_secret    = var.google_oauth_client_secret
  })

  lifecycle {
    # Second line of defense against a partial / rolled-back initial state.
    # The initial stack's outputs.tf precondition is the primary guard; this
    # check fires if the state file is corrupt in a way that bypasses the
    # output precondition (e.g. hand-edited).
    precondition {
      condition     = can(regex("^https://", local.vercel_url))
      error_message = "vercel_url from initial state must be a non-empty https URL. Re-run the initial stack apply before applying settings."
    }
    precondition {
      condition     = length(data.terraform_remote_state.initial.outputs.supabase_project_ref) > 0
      error_message = "supabase_project_ref from initial state is empty. The initial apply did not complete successfully."
    }
  }
}
