output "project_id" {
  description = "Vercel project ID."
  value       = vercel_project.this.id
}

# The vercel provider does not currently expose the auto-assigned production
# hostname as a typed attribute, so the value below is constructed from
# `<project_name>.vercel.app` — the default Vercel assigns for personal-account
# projects whose slug is unique. Team accounts and slug collisions produce
# `<name>-<suffix>.vercel.app` instead, in which case set
# `var.production_domain_override` at the module call site.
output "production_domain" {
  description = "Default production hostname. Override via var.production_domain_override when Vercel assigns a suffixed slug."
  value = (
    var.production_domain_override != ""
    ? var.production_domain_override
    : "${vercel_project.this.name}.vercel.app"
  )
}

output "production_url" {
  description = "Full https URL of the production deployment. Feed back into the supabase settings stack as site_url."
  value = (
    var.production_domain_override != ""
    ? "https://${var.production_domain_override}"
    : "https://${vercel_project.this.name}.vercel.app"
  )
}
