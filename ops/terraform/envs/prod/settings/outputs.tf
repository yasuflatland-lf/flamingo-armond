output "supabase_project_ref" {
  description = "Pass-through of the initial stack's project_ref, surfaced here so operators do not need to switch directories."
  value       = data.terraform_remote_state.initial.outputs.supabase_project_ref
}

output "site_url" {
  description = "Auth Site URL currently configured. Should match the Vercel production URL."
  value       = data.terraform_remote_state.initial.outputs.vercel_production_url
}
