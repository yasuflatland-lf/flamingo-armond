# Backend rewrite contract

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

`frontend/next.config.ts` rewrites `/api/graphql` → `${BACKEND_URL}/query`. This keeps browser requests same-origin (no CORS), matching `backend/cmd/server/main.go` where the Echo router does not register any CORS middleware.

