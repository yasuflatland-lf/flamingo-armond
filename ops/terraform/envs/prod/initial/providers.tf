provider "supabase" {
  access_token = var.supabase_access_token
}

provider "render" {
  api_key  = var.render_api_key
  owner_id = var.render_owner_id
}

provider "vercel" {
  api_token = var.vercel_api_token
  team      = var.vercel_team_id != "" ? var.vercel_team_id : null
}
