# Vercel module

Provisions the Next.js project and its env vars. The default production domain
(`<project>.vercel.app`) is exposed as `production_url` so the supabase
settings stack can wire it into Auth Site URL and the redirect allow list.

## Inputs

| Name | Description |
|---|---|
| `project_name` | Vercel project name. Must match Vercel slug rules. |
| `github_repo` | `owner/name`. The Vercel GitHub App must already be installed. |
| `production_branch` | Branch mapped to production (default `main`). |
| `framework`, `root_directory` | Vercel framework preset and monorepo subdir. |
| `production_domain_override` | Optional. Set when Vercel assigns a suffixed slug (team accounts, name collisions, custom domains). Empty = use default construction. |
| `backend_url` | Render service URL (passes through Next rewrite as `/api/graphql`). |
| `supabase_url`, `supabase_anon_key` | Supabase outputs — exposed to the browser bundle. |

## Outputs

| Name | Description |
|---|---|
| `project_id` | Vercel project ID. |
| `production_domain` | Bare hostname (overridable). |
| `production_url` | Full `https://...` URL — wire into the supabase settings stack as `site_url`. |

## Notes

- `BACKEND_URL` is server-only (no `NEXT_PUBLIC_` prefix). The browser hits
  `/api/graphql` on its own origin; `next.config.ts` rewrites that to
  `${BACKEND_URL}/query`. Marked `sensitive = true` so the value does not
  appear in `terraform plan` output, even though it is not a credential.
- Custom domains are intentionally out of scope. Add a sibling
  `vercel_project_domain` resource at the env layer when needed, then set
  `production_domain_override` so downstream consumers (Supabase Auth) wire to
  the canonical hostname instead of the default `*.vercel.app`.
