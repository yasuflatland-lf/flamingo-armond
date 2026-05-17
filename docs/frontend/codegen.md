# Codegen

> Part of [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md). See the index for related chapters.

Run codegen with:

```bash
pnpm --filter frontend codegen   # frontend only
make codegen                     # repo root — runs backend (gqlgen) + frontend (graphql-codegen)
```

- **Input**: `schema/*.graphql` — shared single source of truth for both sides.
- **Output**: `frontend/src/generated/` (git-ignored). Generated files include `graphql.ts`, `gql.ts`, `fragment-masking.ts`, and `index.ts`.
- **Document discovery**: the client-preset scans `frontend/src/**/*.{ts,tsx}` for `graphql()` tagged templates and includes only the operations actually used.
- **Lifecycle**: a `prebuild` hook in `frontend/package.json` runs codegen automatically before `pnpm build`. In CI, a dedicated `Codegen (graphql-codegen)` step runs between `Install dependencies` and `Biome check`.
- `src/generated/**` is NOT in `tsconfig.json`'s `exclude` — generated output participates in `tsc --noEmit`. If codegen produces broken types, typecheck fails fast rather than hiding behind the exclude.

Usage pattern:

```ts
import { graphql } from "@/generated";
const HealthQuery = graphql(`query Health { health }`);
```

The same generated-files policy applies to `backend/graph/generated/` and `backend/graph/model/models_gen.go` on the Go side.

