output "project_id" {
  description = "Vercel project ID."
  value       = vercel_project.this.id
}

output "production_domain" {
  description = "Default production hostname (e.g. flamingo-armond.vercel.app)."
  # The vercel provider exposes the auto-assigned production domain via this
  # attribute. Custom domains, when added, can be folded in by a sibling
  # vercel_project_domain resource at the env layer.
  value = "${vercel_project.this.name}.vercel.app"
}

output "production_url" {
  description = "Full https URL of the production deployment. Feed back into the supabase module as site_url."
  value       = "https://${vercel_project.this.name}.vercel.app"
}
