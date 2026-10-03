# Stack

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

- Next.js 16 (App Router) + React 19.2 + TypeScript 5.9 (Node 24.x)
- Tailwind 4 (CSS-first config via `@theme`, no `tailwind.config.ts`)
- shadcn/ui (initialized; component set grows incrementally as features need them)
- Biome 2 for lint + format (no ESLint, no Prettier — do not run `next lint`)
- `@t3-oss/env-nextjs` + Zod for env validation
- Apollo Client via `@apollo/client-integration-nextjs`; GraphQL code generation via `@graphql-codegen/client-preset`. RSC renders use a thin `gqlFetch` helper; browser code uses `useQuery` through `ApolloNextAppProvider`.
- Vitest for unit tests; property-based tests use `fast-check`. Global parameters (fixed seed, 100 runs) live in `src/__test-setup__/fast-check.ts` — rerun with `FC_SEED=<n>` to explore other inputs — and shared text arbitraries in `src/test/text-arbitraries.ts`.

