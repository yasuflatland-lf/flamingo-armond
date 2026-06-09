# Env vars

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

| Name | Where validated | Default (example) | Purpose |
|---|---|---|---|
| `BACKEND_URL` | `frontend/src/env.ts` (server) | `http://localhost:1323` | Base URL for the `/api/graphql` rewrite in `next.config.ts`. |
| `NEXT_PUBLIC_SUPABASE_URL` | `frontend/src/env.ts` (client) | `http://127.0.0.1:54321` | Supabase API base URL. Public — bundled into client. |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | `frontend/src/env.ts` (client) | `sb_publishable_...` (the **Publishable** value from `supabase start`; the env name keeps the legacy `_ANON_KEY` to match the Supabase JS SDK) | Public client key. RLS gates real access. |
| `NEXT_PUBLIC_SITE_URL` | `frontend/src/env.ts` (client) | `http://localhost:3000` (optional — defaults when unset) | Canonical site origin for SEO metadata (`metadataBase`, Open Graph URLs). Set to the deployed origin in production. |

