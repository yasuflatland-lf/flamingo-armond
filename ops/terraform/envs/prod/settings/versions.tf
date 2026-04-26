terraform {
  required_version = ">= 1.14, < 2.0"

  required_providers {
    supabase = {
      source  = "supabase/supabase"
      version = "~> 1.5"
    }
  }
}
