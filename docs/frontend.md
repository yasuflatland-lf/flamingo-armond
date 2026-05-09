# Frontend / schema notes

## Chapters

- [Stack](frontend/stack.md) — Next.js 16, React 19.2, TypeScript 5.9, Tailwind 4, shadcn/ui, Biome 2, Apollo Client.
- [Dev quickstart](frontend/dev-quickstart.md) — Env-file setup and the `pnpm --filter frontend` commands for dev, build, lint, typecheck, and e2e.
- [Env vars](frontend/env-vars.md) — Validated env vars (`BACKEND_URL`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`).
- [Backend rewrite contract](frontend/backend-rewrite-contract.md) — `next.config.ts` rewrites `/api/graphql` to the backend so browser requests stay same-origin.
- [E2E tests](frontend/e2e-tests.md) — Playwright specs under `frontend/e2e/` and the service-role-key handling rule.
- [Tailwind 4 notes](frontend/tailwind-4-notes.md) — CSS-first config via `@theme` in `globals.css`, no `tailwind.config.ts`, PostCSS plugin notes.
- [Codegen](frontend/codegen.md) — `graphql-codegen` client-preset wiring, document discovery, and the `prebuild` lifecycle.
- [Apollo wiring](frontend/apollo-wiring.md) — `gqlFetch` for RSC, `ApolloNextAppProvider` for the browser, and cache mutation patterns (create/delete/Connection).
- [Routing topology](frontend/routing-topology.md) — HomePage 4-branch redirect, `/cards/new` resolution chain, global nav primitives, login/cardgroup-create flows.
- [Route Handler conventions](frontend/route-handler-conventions.md) — Per-route layout for `app/api/**/route.ts`, discriminated-union responses, the `/api/healthz` JSON probe.
- [Auth (Supabase)](frontend/auth-supabase.md) — 3-layer Supabase SSR client, `authLink`, middleware cookie rotation, and known gotchas.
- [Profile page (`/profile`)](frontend/profile-page-profile.md) — RSC + client form pattern, admin layout gate, form library conventions, and field-error surfacing.
- [shadcn/ui](frontend/shadcnui.md) — Committed `components.json`/`cn()` helper, current component set, and the non-interactive bootstrap fallback.
- [Backend error-code contract](frontend/backend-error-code-contract.md) — Three `extensions.code` values (`BAD_USER_INPUT`, `UNAUTHENTICATED`, `INTERNAL`) and the frontend handlers per layer.
- [Shared helper modules](frontend/shared-helper-modules.md) — Import contracts for the field-error / banner / redirect / format / FieldError / grapheme-count helpers.
- [Observability](frontend/observability.md) — Pointer to `docs/observability.md` plus frontend request-ID generator and Apollo link/RSC propagation.
- [Gotchas encountered](frontend/gotchas-encountered.md) — Cross-cutting Next.js, Apollo, Vitest, Biome, and bundler pitfalls accumulated during development.
- [Automatic Persisted Queries](frontend/automatic-persisted-queries.md) — Browser link-chain order, native WebCrypto sha256, `useGETForHashedQueries: false` rationale, and link-chain test gotchas.
- [Testing convention: narrow vs broad page tests](frontend/testing-convention-narrow-vs-broad-page-tests.md) — Narrow vs broad test naming contract, shared utilities, RSC test pattern, Apollo v4 migration notes.
- [Delayed-DELETE undo toast, SwipeableRow, and useReducedMotion](frontend/delayed-delete-undo-toast-swipeablerow-and-usereducedmotion.md) — Pointer to `docs/frontend-undo-toast.md` plus a coverage-migration cautionary tale.
