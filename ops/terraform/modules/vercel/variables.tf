variable "project_name" {
  description = "Vercel project name."
  type        = string
  validation {
    # Vercel slug rules: lowercase letters, digits, hyphens, no leading/trailing hyphen.
    condition     = can(regex("^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$", var.project_name))
    error_message = "project_name must match Vercel's slug rules: lowercase letters, digits, and internal hyphens only."
  }
}

variable "github_repo" {
  description = "GitHub repo slug (owner/name). The Vercel GitHub App must already be installed on the repo."
  type        = string
  validation {
    condition     = can(regex("^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$", var.github_repo))
    error_message = "github_repo must be in the form owner/name."
  }
}

variable "production_branch" {
  description = "Branch that maps to the production deployment."
  type        = string
  default     = "main"
}

variable "framework" {
  description = "Vercel framework preset."
  type        = string
  default     = "nextjs"
}

variable "root_directory" {
  description = "Project root inside the monorepo."
  type        = string
  default     = "frontend"
}

variable "production_domain_override" {
  description = <<-EOT
    Override for the production hostname when Vercel does not assign
    `<project_name>.vercel.app` directly (e.g. team accounts that suffix the
    slug, or projects with a custom domain). Leave empty to use the default
    construction. Example: "flamingo-armond-team.vercel.app".
  EOT
  type        = string
  default     = ""
  validation {
    # Commas would split the value when the settings stack joins redirect URLs
    # for Supabase Auth, silently expanding the allow list.
    condition     = !strcontains(var.production_domain_override, ",")
    error_message = "production_domain_override must not contain commas (it is interpolated into a comma-separated allow list)."
  }
}

variable "backend_url" {
  description = "Render service URL. Read at build time by next.config.ts and at runtime by the RSC gqlFetch helper."
  type        = string
  validation {
    condition     = can(regex("^https://", var.backend_url))
    error_message = "backend_url must be an https URL."
  }
}

variable "supabase_url" {
  description = "Supabase project URL. Public — exposed via NEXT_PUBLIC_SUPABASE_URL."
  type        = string
  validation {
    condition     = can(regex("^https://", var.supabase_url))
    error_message = "supabase_url must be an https URL."
  }
}

variable "supabase_anon_key" {
  description = "Supabase publishable / anon key. Public — exposed via NEXT_PUBLIC_SUPABASE_ANON_KEY."
  type        = string
  sensitive   = true
  validation {
    condition     = length(var.supabase_anon_key) > 0
    error_message = "supabase_anon_key must not be empty."
  }
}
