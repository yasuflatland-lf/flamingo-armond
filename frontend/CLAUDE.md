# frontend/

Next.js 16 / React 19.2 / Apollo Client / TypeScript 5.9. The L1 (`CLAUDE.md` at repo root) applies; this file adds frontend-specific orientation.

## Layout and commands

Run from the repo root (pnpm workspace):

```bash
cp frontend/.env.example frontend/.env.local
pnpm install
pnpm --filter frontend dev        # http://localhost:3000
pnpm --filter frontend build
pnpm --filter frontend lint
pnpm --filter frontend typecheck
pnpm --filter frontend test
pnpm --filter frontend e2e
```

The frontend package manager is **pnpm** (pinned via `package.json` engines + `packageManager`); do not invent commands with `npm`/`yarn`.

## Topic docs

- [`docs/frontend/stack.md`](../docs/frontend/stack.md) — Next.js 16, React 19.2, TypeScript 5.9, Tailwind 4, shadcn/ui, Biome 2, Apollo Client.
- [`docs/frontend/dev-quickstart.md`](../docs/frontend/dev-quickstart.md) — Env-file setup and `pnpm --filter frontend` commands.
- [`docs/frontend/env-vars.md`](../docs/frontend/env-vars.md) — Validated env vars (`BACKEND_URL`, `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY`).
- [`docs/frontend/backend-rewrite-contract.md`](../docs/frontend/backend-rewrite-contract.md) — `/api/graphql` rewrite contract.
- [`docs/frontend/e2e-tests.md`](../docs/frontend/e2e-tests.md) — Playwright + service-role-key handling.
- [`docs/frontend/tailwind-4-notes.md`](../docs/frontend/tailwind-4-notes.md) — CSS-first config via `@theme`, no `tailwind.config.ts`.
- [`docs/frontend/design-system.md`](../docs/frontend/design-system.md) — Color-token semantics (brand/destructive/primary), `Button` variant usage, and the destructive-confirm-dialog convention.
- [`docs/frontend/codegen.md`](../docs/frontend/codegen.md) — `graphql-codegen` wiring and `prebuild` lifecycle.
- [`docs/frontend/apollo-wiring.md`](../docs/frontend/apollo-wiring.md) — `gqlFetch` for RSC and Apollo client cache patterns.
- [`docs/frontend/routing-topology.md`](../docs/frontend/routing-topology.md) — HomePage / cardgroup / login / `/cards/new` chains.
- [`docs/frontend/onboarding-gate.md`](../docs/frontend/onboarding-gate.md) — `displayName`-required gate ownership.
- [`docs/frontend/route-handler-conventions.md`](../docs/frontend/route-handler-conventions.md) — Per-route layout for `app/api/**/route.ts`.
- [`docs/frontend/auth-supabase.md`](../docs/frontend/auth-supabase.md) — Supabase SSR client and middleware cookie rotation.
- [`docs/frontend/csp.md`](../docs/frontend/csp.md) — CSP builders, middleware nonce flow, static headers, and violation reporting.
- [`docs/frontend/pwa.md`](../docs/frontend/pwa.md) — Installable PWA, static service worker, auth-safe fetch guards, and version-stamped update propagation.
- [`docs/frontend/profile-page-profile.md`](../docs/frontend/profile-page-profile.md) — `/profile` RSC + client form.
- [`docs/frontend/i18n.md`](../docs/frontend/i18n.md) — next-intl (en/ja), cookie locale resolution, message catalogs, `AppConfig` augmentation, and the `renderWithIntl` test harness.
- [`docs/frontend/shadcnui.md`](../docs/frontend/shadcnui.md) — Committed `components.json` and `cn()` helper.
- [`docs/frontend/backend-error-code-contract.md`](../docs/frontend/backend-error-code-contract.md) — `extensions.code` handlers per layer.
- [`docs/frontend/shared-helper-modules.md`](../docs/frontend/shared-helper-modules.md) — Field-error / banner / redirect / format helpers.
- [`docs/frontend/observability.md`](../docs/frontend/observability.md) — Frontend request-ID and Apollo propagation.
- [`docs/frontend/gotchas-encountered.md`](../docs/frontend/gotchas-encountered.md) — Cross-cutting Next.js / Apollo pitfalls.
- [`docs/frontend/automatic-persisted-queries.md`](../docs/frontend/automatic-persisted-queries.md) — Browser link chain and APQ.
- [`docs/frontend/testing-convention-narrow-vs-broad-page-tests.md`](../docs/frontend/testing-convention-narrow-vs-broad-page-tests.md) — Narrow vs broad page tests.
- [`docs/frontend/mutation-testing.md`](../docs/frontend/mutation-testing.md) — Stryker mutation testing on `src/lib/**` (`test:mutation`); report-only, nightly/manual CI.
- [`docs/frontend/typescript-conventions.md`](../docs/frontend/typescript-conventions.md) — Type-design rules: required nullable props, JSDoc sanitization contracts, assertion discriminating keys, TanStack Form re-throw.
- [`docs/frontend/undo-toast.md`](../docs/frontend/undo-toast.md) — Delayed-DELETE undo toast, SwipeableRow, `useReducedMotion`.
- [`docs/frontend/url-backed-sheet-state.md`](../docs/frontend/url-backed-sheet-state.md) — `useSheetSearchParam` drawer state in `?new=true` / `?edit=<id>`; singleton sentinel, lazy-query called gate, race guard.

## Cross-cutting rules already in context

RSC error handling, TypeScript conventions, pagination patterns are auto-loaded via `.claude/rules/`. Do not duplicate them here.
