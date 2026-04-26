terraform {
  required_version = ">= 1.14, < 2.0"

  required_providers {
    supabase = {
      source  = "supabase/supabase"
      version = "~> 1.5"
    }
    render = {
      source  = "render-oss/render"
      version = "~> 1.0"
    }
    vercel = {
      source  = "vercel/vercel"
      version = "~> 2.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}
