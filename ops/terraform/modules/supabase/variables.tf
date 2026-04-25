variable "project_name" {
  description = "Display name for the Supabase project."
  type        = string
  validation {
    condition     = length(var.project_name) > 0
    error_message = "project_name must not be empty."
  }
}

variable "organization_id" {
  description = "Supabase organization ID that owns the project."
  type        = string
  validation {
    condition     = length(var.organization_id) > 0
    error_message = "organization_id must not be empty."
  }
}

variable "region" {
  description = "Supabase region slug (e.g. ap-northeast-1, us-east-1, eu-west-1)."
  type        = string
  validation {
    # AWS-style slug: two-letter geo, region word, single-digit suffix.
    condition     = can(regex("^[a-z]{2}-[a-z]+-[1-9][0-9]?$", var.region))
    error_message = "region must be a Supabase region slug shaped like ap-northeast-1."
  }
}
