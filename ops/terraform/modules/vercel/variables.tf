variable "project_name" {
  description = "Vercel project name."
  type        = string
}

variable "github_repo" {
  description = "GitHub repo slug (owner/name). The Vercel GitHub App must already be installed on the repo."
  type        = string
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

variable "backend_url" {
  description = "Render service URL. Read at build time by next.config.ts and at runtime by the RSC gqlFetch helper."
  type        = string
}

variable "supabase_url" {
  description = "Supabase project URL. Public — exposed via NEXT_PUBLIC_SUPABASE_URL."
  type        = string
}

variable "supabase_anon_key" {
  description = "Supabase publishable / anon key. Public — exposed via NEXT_PUBLIC_SUPABASE_ANON_KEY."
  type        = string
  sensitive   = true
}
