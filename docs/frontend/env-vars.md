# Env vars

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

| Name | Where validated | Default (example) | Purpose |
|---|---|---|---|
| `BACKEND_URL` | `frontend/src/env.ts` (server) | `http://localhost:1323` | Base URL for the `/api/graphql` rewrite in `next.config.ts`. Also the endpoint the middleware onboarding gate queries on a cookie miss. |
| `ONBOARDING_GATE_SECRET` | `frontend/src/env.ts` (server) | unset (optional; min 32 chars) | HMAC key for the middleware onboarding gate's `fa-onboarded` fast-path cookie. Unset is safe — the gate then re-checks the display name against the backend on every gated navigation instead of trusting a cookie it cannot authenticate. Generate with `openssl rand -base64 32`; use a distinct value per environment. See [`onboarding-gate.md`](./onboarding-gate.md). |
| `NEXT_PUBLIC_SUPABASE_URL` | `frontend/src/env.ts` (client) | `http://127.0.0.1:54321` | Supabase API base URL. Public — bundled into client. |
| `NEXT_PUBLIC_SUPABASE_ANON_KEY` | `frontend/src/env.ts` (client) | `sb_publishable_...` (the **Publishable** value from `supabase start`; the env name keeps the legacy `_ANON_KEY` to match the Supabase JS SDK) | Public client key. RLS gates real access. |
| `NEXT_PUBLIC_SITE_URL` | `frontend/src/env.ts` (client) | `http://localhost:3000` in development/test; falls back to `https://<VERCEL_URL>` on Vercel Preview; **required** in production (throws at build time when `NODE_ENV=production` and `VERCEL_ENV=production` and this is unset) | Canonical site origin for SEO metadata (`metadataBase`, Open Graph URLs). Registered for Production only by `make setup-prod` Phase 4 (`playbooks/setup-prod/vercel.yml`) from the operator-entered production URL. It is left unset on Preview because a registered value would take precedence over the `VERCEL_URL` fallback; see [`../deployment.md` § "Step 3 — Vercel"](../deployment.md#step-3--vercel). |

