# Vercel module

Provisions the Next.js project and its env vars. The default production domain
(`<project>.vercel.app`) is exposed as `production_url` so the supabase module
can wire it into Auth Site URL and the redirect allow list.

## Inputs

| Name | Description |
|---|---|
| `project_name` | Vercel project name. |
| `github_repo` | `owner/name`. The Vercel GitHub App must already be installed. |
| `production_branch` | Branch mapped to production (default `main`). |
| `framework`, `root_directory` | Vercel framework preset and monorepo subdir. |
| `backend_url` | Render service URL (passes through Next rewrite as `/api/graphql`). |
| `supabase_url`, `supabase_anon_key` | Supabase outputs — exposed to the browser bundle. |

## Outputs

| Name | Description |
|---|---|
| `project_id` | Vercel project ID. |
| `production_domain` | Bare hostname. |
| `production_url` | Full `https://...` URL — wire into supabase module's `site_url`. |

## Notes

- `BACKEND_URL` is server-only (no `NEXT_PUBLIC_` prefix). The browser hits
  `/api/graphql` on its own origin; `next.config.ts` rewrites that to
  `${BACKEND_URL}/query`.
- Custom domains are intentionally out of scope. Add a sibling
  `vercel_project_domain` resource at the env layer when needed.
