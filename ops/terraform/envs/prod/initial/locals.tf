locals {
  # Production keeps simple names. When staging is added, that env's locals.tf
  # appends a suffix (e.g. "-staging") to keep all three providers' names
  # disambiguated.
  supabase_project_name = var.project_name
  render_service_name   = "${var.project_name}-backend"
  vercel_project_name   = var.project_name
}
