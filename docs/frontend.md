# Frontend / schema notes

## Stack

- Next.js 16 (App Router) + React 19.2 + TypeScript 5.9 (Node 24.x)
- Tailwind 4 (CSS-first config via `@theme`, no `tailwind.config.ts`)
- shadcn/ui (initialized; actual components land in PR6)
- Biome 2 for lint + format (no ESLint, no Prettier — do not run `next lint`)
- `@t3-oss/env-nextjs` + Zod for env validation
- Apollo Client / graphql-codegen are deferred to PR5 — `frontend/codegen.ts` is intentionally empty.

## Dev quickstart

```bash
cp frontend/.env.example frontend/.env.local   # tweak BACKEND_URL if needed
pnpm --filter frontend dev                     # http://localhost:3000
pnpm --filter frontend build                   # production build
pnpm --filter frontend lint                    # biome check .
pnpm --filter frontend typecheck               # tsc --noEmit
```

## Env vars

| Name | Where validated | Default (example) | Purpose |
|---|---|---|---|
| `BACKEND_URL` | `frontend/src/env.ts` (server) | `http://localhost:1323` | Base URL for the `/api/graphql` rewrite in `next.config.ts`. |

`NEXT_PUBLIC_SUPABASE_URL` / `NEXT_PUBLIC_SUPABASE_ANON_KEY` arrive in PR6.

## Backend rewrite contract

`frontend/next.config.ts` rewrites `/api/graphql` → `${BACKEND_URL}/query`. This keeps browser requests same-origin (no CORS), matching `backend/cmd/server/main.go` where `handler.New` does not add a CORS transport.

## Tailwind 4 notes

- Tokens and dark-mode variant live in `src/app/globals.css` via `@theme` and `@custom-variant dark (...)`. There is no `tailwind.config.ts`.
- PostCSS plugin is `@tailwindcss/postcss` (the old `tailwindcss` plugin no longer exists in v4). `autoprefixer` is not needed — Tailwind 4 handles prefixing internally.
- When PR6 adds shadcn components, extend the `@theme` block with the additional `--color-*` tokens the components reference.

## Codegen — generated files are NOT committed

`frontend/codegen.ts` is a 0-byte placeholder. PR5 populates it to consume `schema/*.graphql` and emit into `frontend/src/generated/`. **That output is gitignored** (see `docs/dev-setup.md` §"Policy on generated files"); each environment regenerates before `pnpm build`. The same policy applies to `backend/graph/generated/` and `backend/graph/model/models_gen.go` on the Go side.

Until PR5 lands, do **not** run `pnpm --filter frontend codegen` — the script is missing and pnpm exits 1 by design.

## shadcn/ui

`frontend/components.json` and `frontend/src/lib/utils.ts` (the `cn()` helper) are committed. No components are added yet. PR6 runs `pnpm dlx shadcn add button input label form` and extends `globals.css` with the theme tokens those components reference.
