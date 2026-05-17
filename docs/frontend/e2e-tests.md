# E2E tests

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

Playwright specs live in `frontend/e2e/` and use `frontend/playwright.config.ts` (which builds the app and serves it via `next start`). Run `pnpm --filter frontend test:e2e` after starting Supabase and the Go backend. The service-role key must stay in `E2E_SUPABASE_SERVICE_ROLE_KEY` and never be exposed as `NEXT_PUBLIC_*`. See `docs/e2e.md` for the auth helper, local run setup, and CI flow.

